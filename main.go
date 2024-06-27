package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/beclab/l4-bfl-proxy/util"
	appv2alpha1 "github.com/beclab/l4-bfl-proxy/util/app/v2alpha1"
	"github.com/beclab/l4-bfl-proxy/util/nginx"
	"github.com/beclab/l4-bfl-proxy/util/signal"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	iamv1alpha2 "kubesphere.io/api/iam/v1alpha2"
	ctrl "sigs.k8s.io/controller-runtime"
)

//go:embed config/nginx.tmpl
var ngxTemplateContent string

var (
	verbose bool

	enableNginx bool

	inContainer bool

	userNamespacePrefix string

	workerProcesses int

	sslServerPort int

	bflServicePort int

	periodSeconds uint
)

var (
	annotationGroup = "bytetrade.io"

	userAnnotationDid = fmt.Sprintf("%s/did", annotationGroup)

	userAnnotationZone = fmt.Sprintf("%s/zone", annotationGroup)

	userLauncherAccessLevel = fmt.Sprintf("%s/launcher-access-level", annotationGroup)

	userLauncherAllowCIDR = fmt.Sprintf("%s/launcher-allow-cidr", annotationGroup)

	userAnnotationCreator = "kubesphere.io/creator"

	userAnnotationIsEphemeral = fmt.Sprintf("%s/is-ephemeral", annotationGroup)

	userDenyAllPolicy = fmt.Sprintf("%s/deny-all", annotationGroup)

	userAllowedDomainAccessPolicy = fmt.Sprintf("%s/allowed-domains", annotationGroup)

	iamUserGVR = schema.GroupVersionResource{
		Group:    "iam.kubesphere.io",
		Version:  "v1alpha2",
		Resource: "users",
	}

	appGVR = schema.GroupVersionResource{
		Group:    "app.bytetrade.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	luaNgxStreamPort = 2444

	luaNgxStreamAPIAddress = fmt.Sprintf("127.0.0.1:%d", luaNgxStreamPort)

	sslProxyServerPort                   = 444
	settingsCustomDomain                 = "customDomain"
	settingsCustomDomainThirdLevelDomain = "third_level_domain"
	settingsCustomDomainThirdPartyDomain = "third_party_domain"

	ApplicationAuthorizationLevelPrivate = "private"
	ApplicationAuthorizationLevelPublic  = "public"
)

type User struct {
	Name                 string   `json:"name"`
	Namespace            string   `json:"namespace"`
	Did                  string   `json:"did"`
	Zone                 string   `json:"zone"`
	IsEphemeral          string   `json:"is_ephemeral"`
	BFLIngressSvcHost    string   `json:"bfl_ingress_svc_host"`
	BFLIngressSvcPort    int      `json:"bfl_ingress_svc_port"`
	AccessLevel          uint64   `json:"access_level"`
	AllowCIDRs           []string `json:"allow_cidrs"`
	DenyAll              int      `json:"deny_all"`
	AllowedDomains       []string `json:"allowed_domains"`
	NgxServerNameDomains []string `json:"ngx_server_name_domains"`
	CreateTimestamp      int64    `json:"create_timestamp"`
}

type Users []User

func (u Users) Len() int           { return len(u) }
func (u Users) Less(i, j int) bool { return u[i].CreateTimestamp > u[j].CreateTimestamp }
func (u Users) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }

type Cfg struct {
	WorkerProcesses    int
	StreamAPIAddress   string
	SSLProxyServerPort int
	SSLServerPort      int
	BFLServerPort      int
}

type Server struct {
	client dynamic.Interface

	Cfg *Cfg

	Users Users

	ngxCmd *nginx.Command

	ngxTmpl *template.Template
}

func (s *Server) init() error {
	klog.Info("init kubernetes clientset")
	config, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	s.client = client

	// nginx command
	s.ngxCmd = nginx.NewCommand()

	// tmpl
	tmpl, err := template.New("nginx.tmpl").Parse(ngxTemplateContent)
	if err != nil {
		return err
	}
	s.ngxTmpl = tmpl

	// cfg
	s.Cfg = &Cfg{
		WorkerProcesses:    workerProcesses,
		StreamAPIAddress:   luaNgxStreamAPIAddress,
		SSLServerPort:      sslServerPort,
		SSLProxyServerPort: sslProxyServerPort,
	}

	klog.Info("ensure nginx processes is running")
	if err = s.startNgx(); err != nil {
		return err
	}

	return nil
}

func (s *Server) waitForNgxStartup() bool {
	isRunning := false
	timeoutForWait := time.After(120 * time.Second)

	for {
		if nginx.IsRunning() {
			isRunning = true
			break
		}
		select {
		case <-timeoutForWait:
			klog.Warning("waiting for nginx startup timed out!")
		default:
			time.Sleep(time.Second)
		}
	}
	return isRunning
}

func (s *Server) startNgx() error {
	if !inContainer {
		klog.Warning("not in container, ignore")
		return nil
	}

	if nginx.IsRunning() {
		klog.Warning("nginx process is running, ignore")
		return nil
	}

	// validate config file
	klog.Info("testing nginx, using default /etc/nginx/nginx.conf")
	testOut, err := s.ngxCmd.Test("")
	if err != nil {
		return fmt.Errorf("%v\n%v", err, string(testOut))
	}

	klog.Infof("starting nginx processes")
	go s.ngxCmd.Start()
	//if err != nil {
	//	return fmt.Errorf("%v\n%v", err, string(out))
	//}
	return nil
}

func (s *Server) waitForStreamLuaPort() error {
	timeout := time.After(120 * time.Second)

	dial := func() error {
		_, err := net.DialTimeout("tcp", luaNgxStreamAPIAddress, 2*time.Second)
		if err != nil {
			return err
		}
		return nil
	}

	for {
		select {
		case <-timeout:
			return fmt.Errorf("120 seconds timed out to wait stream server listen")
		default:
			time.Sleep(time.Second)
		}

		if err := dial(); err == nil {
			return nil
		} else {
			klog.Warningf("dial stream server err, %v", err)
		}
	}
}

func (s *Server) writeLuaConfig(users Users) error {
	var err error
	if users == nil {
		users, err = s.listUsers()
		if err != nil {
			return fmt.Errorf("write lua config: list users err, %v", err)
		}
	}
	if s.Users != nil && reflect.DeepEqual(s.Users, users) {
		klog.V(2).Infof("write lua config, users no changed, ignore update it")
		return nil
	}
	s.Users = users

	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.Dial("tcp", luaNgxStreamAPIAddress)
	if err != nil {
		return fmt.Errorf("write lua config: connect to lua nginx tcp server err, %v", err)
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("write lua config: unexpected type *net.TCPConn")
	}
	defer tcpConn.Close()

	if err = tcpConn.SetWriteBuffer(128 * 1024); err != nil {
		return fmt.Errorf("write lua config: set write buffer err, %v", err)
	}

	var payload []byte

	payload, err = json.Marshal(s.Users)
	if err != nil {
		return fmt.Errorf("write lua config: marshal users err, %v", err)
	} else {
		klog.Infof("write lua config: payload data: %s", string(payload))
		tcpConn.Write(payload)
		tcpConn.Write([]byte("\r\n"))
	}
	return nil
}

func (s *Server) watchUser(stop <-chan struct{}, timeAfter time.Duration) {
	time.Sleep(timeAfter)

	userWatch, err := s.client.Resource(iamUserGVR).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("watch user err, %v", err)
		return
	}
	defer userWatch.Stop()

	for {
		select {
		case event, ok := <-userWatch.ResultChan():
			if !ok {
				userWatch.Stop()
				time.Sleep(time.Second)
				userWatch, err = s.client.Resource(iamUserGVR).Watch(context.TODO(), metav1.ListOptions{})
				if err != nil {
					klog.V(2).Infof("re-watch iam users err: %s", err)
				}
				continue
			}
			klog.V(2).Infof("watch user: received event, %v", event.Type)
			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				if err := s.writeLuaConfig(nil); err != nil {
					klog.V(2).Infof("watch user, event type: %v, err: %v", event.Type, err)
				}
			}
		case <-stop:
			return
		}
	}
}

func (s *Server) lookupHostAddr(svc string) (string, error) {
	var maxRetry = 15

	for ; maxRetry > 0; maxRetry-- {
		addr, err := net.LookupHost(svc)
		if err != nil {
			klog.V(2).Infof("svc %s, lookup host err, %v", svc, err)
			time.Sleep(3 * time.Second)
		}

		if len(addr) >= 1 {
			return addr[0], nil
		}
	}

	return "", fmt.Errorf("svc %s, no host lookup", svc)
}

func (s *Server) listApplications() ([]string, []string, []string) {
	var publicApps []string
	var publicCustomDomainApps []string
	var customDomainApps []string

	list, err := s.client.Resource(appGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil
	}

	data, err := list.MarshalJSON()
	if err != nil {
		return nil, nil, nil
	}

	var appList appv2alpha1.ApplicationList
	if err = json.Unmarshal(data, &appList); err != nil {
		return nil, nil, nil
	}

	getAppPrefix := func(entrancecount, index int, appid string) string {
		if entrancecount == 1 {
			return appid
		}
		return fmt.Sprintf("%s%d", appid, index)
	}

	for _, app := range appList.Items {
		if app.Spec.Entrances == nil || len(app.Spec.Entrances) == 0 {
			continue
		}

		var customDomains []string
		var customDomainsPrefix []string
		var entrancecounts = len(app.Spec.Entrances)

		for index, entrance := range app.Spec.Entrances {
			prefix := getAppPrefix(entrancecounts, index, app.Spec.Appid)

			customDomainEntrancesMap := getSettingsKeyMap(&app, settingsCustomDomain)
			entranceAuthorizationLevel := entrance.AuthLevel

			customDomainEntrance, ok := customDomainEntrancesMap[entrance.Name]
			if ok {
				if entrancePrefix := customDomainEntrance[settingsCustomDomainThirdLevelDomain]; entrancePrefix != "" {
					customDomainsPrefix = append(customDomainsPrefix, entrancePrefix)
				}
				if entranceCustomDomain := customDomainEntrance[settingsCustomDomainThirdPartyDomain]; entranceCustomDomain != "" {
					customDomainApps = append(customDomainApps, entranceCustomDomain)

					if entranceAuthorizationLevel == ApplicationAuthorizationLevelPublic {
						customDomains = append(customDomains, entranceCustomDomain)
					}
				}
			}

			if prefix != "" {
				publicApps = append(publicApps, prefix)
				if len(customDomainsPrefix) > 0 {
					publicApps = append(publicApps, customDomainsPrefix...)
				}

				if len(customDomains) > 0 {
					publicCustomDomainApps = append(publicCustomDomainApps, customDomains...)
				}
			}
		}
	}

	return publicApps, publicCustomDomainApps, customDomainApps
}

func (s *Server) listUsers() (Users, error) {
	publicAppIdList, publicCustomDomainAppList, customDomainAppList := s.listApplications()

	list, err := s.client.Resource(iamUserGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	data, err := list.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var userList iamv1alpha2.UserList
	if err = json.Unmarshal(data, &userList); err != nil {
		return nil, err
	}

	getUserByName := func(name string) *iamv1alpha2.User {
		for _, user := range userList.Items {
			if user.Name == name {
				return &user
			}
		}
		return nil
	}

	getPublicAccessDomain := func(zone string, publicAppIds []string, publicCustomDomainApps []string, denied string) []string {
		var r []string
		if (publicAppIds == nil && publicCustomDomainApps == nil) || denied != "1" {
			return r
		}

		for _, appId := range publicAppIds {
			r = append(r, appId+"."+zone)
		}

		for _, appId := range publicCustomDomainApps {
			r = append(r, appId)
		}

		return r
	}

	users := make(Users, 0)

	for _, user := range userList.Items {
		isEphemeral := getUserAnnotation(&user, userAnnotationIsEphemeral)

		if !isValidUser(&user) && isEphemeral == "" {
			if verbose {
				klog.Warningf("ignore invalid user '%s', no did/zone and is-ephemeral annotation", user.Name)
			}
			continue
		}

		var isEphemeralDomain = "no"

		if ok, err := strconv.ParseBool(isEphemeral); err == nil && ok {
			isEphemeralDomain = "yes"
		}

		var (
			did, zone            string
			accLevel, allowCIDR  string
			denyAllStatus        string
			allowedDomainsAnno   []string
			ngxServerNameDomains []string
		)

		if isEphemeralDomain == "no" {
			did, zone = getUserAnnotation(&user, userAnnotationDid), getUserAnnotation(&user, userAnnotationZone)
			accLevel = getUserAnnotation(&user, userLauncherAccessLevel)
			allowCIDR = getUserAnnotation(&user, userLauncherAllowCIDR)
			ngxServerNameDomains = []string{"local." + zone, zone}
			denyAllStatus = getUserAnnotation(&user, userDenyAllPolicy)
			allowedDomainsAnno = getPublicAccessDomain(zone, publicAppIdList, publicCustomDomainAppList, denyAllStatus)

			if customDomainAppList != nil && len(customDomainAppList) > 0 {
				ngxServerNameDomains = append(ngxServerNameDomains, customDomainAppList...)
			}
		} else {
			// creator user
			creator := getUserAnnotation(&user, userAnnotationCreator)
			creatorUser := getUserByName(creator)
			if creatorUser == nil {
				klog.Warningf("user '%s' is not ephemeral and no creator user", user.Name)
				continue
			}
			did, zone = getUserAnnotation(creatorUser, userAnnotationDid), getUserAnnotation(creatorUser, userAnnotationZone)
			accLevel = getUserAnnotation(creatorUser, userLauncherAccessLevel)
			allowCIDR = getUserAnnotation(creatorUser, userLauncherAllowCIDR)
			denyAllStatus = getUserAnnotation(creatorUser, userDenyAllPolicy)
		}

		var accessLevel uint64

		if accLevel != "" {
			accessLevel, err = strconv.ParseUint(accLevel, 10, 64)
			if err != nil {
				klog.Errorf("user '%s' parse access level uint err, %v", user.Name, err)
				continue
			}
		}

		var denyAll int
		denyAll, _ = strconv.Atoi(denyAllStatus)

		svcName := fmt.Sprintf("bfl.%s-%s", userNamespacePrefix, user.Name)
		addr, err := s.lookupHostAddr(svcName)
		if err != nil {
			klog.V(2).Infof("list user: lookup svc host err, %v", err)
			continue
		}

		_user := User{
			Name:                 user.Name,
			Namespace:            fmt.Sprintf("%s-%s", userNamespacePrefix, user.Name),
			BFLIngressSvcHost:    addr,
			BFLIngressSvcPort:    bflServicePort,
			Did:                  did,
			Zone:                 zone,
			IsEphemeral:          isEphemeralDomain,
			NgxServerNameDomains: ngxServerNameDomains,
			AccessLevel:          accessLevel,
			AllowCIDRs:           strings.Split(allowCIDR, ","),
			DenyAll:              denyAll,
			AllowedDomains:       allowedDomainsAnno,
			CreateTimestamp:      user.CreationTimestamp.Unix(),
		}
		users = append(users, _user)
	}

	// sorted users by createTimestamp desc
	sort.Sort(users)

	return users, nil
}

func (s *Server) testTemplate(data []byte) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("invalid NGINX configuration (empty)")
	}

	var tempNginxPattern = "nginx-cfg"

	tmpFile, err := ioutil.TempFile("", tempNginxPattern)
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	err = ioutil.WriteFile(tmpFile.Name(), data, nginx.PermReadWriteByUser)
	if err != nil {
		return err
	}

	out, err := s.ngxCmd.Test(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("%v\n%v", err, string(out))
	}

	return os.Remove(tmpFile.Name())
}

func (s *Server) render() error {
	klog.Info("generate nginx config")

	var err error

	var buf bytes.Buffer
	if err = s.ngxTmpl.Execute(&buf, s); err != nil {
		return messageWithError("generate nginx config", err)
	}
	content := buf.Bytes()

	klog.Infof("generated nginx configuration content:\n%v", string(content))

	if !inContainer {
		klog.Infof("not in container, ignore")
		return nil
	}

	klog.Info("testing nginx config using temp file")
	if err = s.testTemplate(content); err != nil {
		return messageWithError("testing nginx", err)
	}

	klog.Infof("write nginx content to %s", nginx.DefNgxCfgPath)
	err = ioutil.WriteFile(nginx.DefNgxCfgPath, content, nginx.PermReadWriteByUser)
	if err != nil {
		return messageWithError("write nginx content", err)
	}

	klog.Infof("reloading nginx processes")
	out, err := s.ngxCmd.Reload()
	if err != nil {
		return messageWithError("reload nginx", fmt.Errorf("%v\n%v", err, string(out)))
	}
	klog.Info("reload nginx successfully")

	klog.Info("list users, and write to lua server")
	users, err := s.listUsers()
	if err != nil {
		return messageWithError("write lua server", fmt.Errorf("list users, %v", err))
	}

	if err = s.waitForStreamLuaPort(); err != nil {
		return messageWithError("wait stream lua port listen", err)
	}

	if err = s.writeLuaConfig(users); err != nil {
		return messageWithError("first to write lua server", err)
	}

	return nil
}

func (s *Server) run() {
	if verbose {
		klog.Info("run task, listing iam users ...")
	}

	users, err := s.listUsers()
	if err != nil && verbose {
		klog.Errorf("listing iam users err, %v", err)
		return
	}

	if len(users) == 0 && verbose {
		klog.Warning("no users found.")
		return
	}
	if verbose {
		klog.Infof("got iam users list: %v", util.PrettyJSON(users))
	}

	// render and reload nginx, if users list is changed
	if reflect.DeepEqual(s.Users, users) {
		if verbose {
			klog.Warning("users data not changed, ignore render nginx config")
		}
		return
	}

	s.Users = users

	klog.Info("render to nginx.conf, and reload nginx processes")
	if err = s.render(); err != nil {
		klog.Error(err)
	}
}

func getUserAnnotation(user *iamv1alpha2.User, key string) string {
	if v, ok := user.Annotations[key]; ok && v != "" {
		return v
	}
	return ""
}

func getAppSetting(app *appv2alpha1.Application, key string) string {
	if app.Spec.Settings == nil {
		return ""
	}
	return app.Spec.Settings[key]
}

func getSettingsKeyMap(app *appv2alpha1.Application, key string) map[string]map[string]string {
	var r = make(map[string]map[string]string)
	if app.Spec.Settings == nil {
		return r
	}
	var data = app.Spec.Settings[key]
	if data == "" {
		return r
	}
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return r
	}
	return r
}

func isValidUser(user *iamv1alpha2.User) bool {
	did, zone := getUserAnnotation(user, userAnnotationDid), getUserAnnotation(user, userAnnotationZone)
	return did != "" && zone != ""
}

func init() {
	klog.InitFlags(nil)
}

func flags() error {
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose")
	pflag.BoolVarP(&enableNginx, "enable-nginx", "", true, "Enable gninx process")
	pflag.BoolVarP(&inContainer, "in-container", "", true, "Run in container")
	pflag.StringVarP(&userNamespacePrefix, "user-namespace-prefix", "", "user-space", "User namespace name prefix")
	pflag.IntVarP(&workerProcesses, "nginx-workers", "w", runtime.NumCPU(), "Nginx worker processes")
	pflag.IntVarP(&sslServerPort, "ssl-server-port", "", 443, "Stream ssl proxy listen port")
	pflag.IntVarP(&bflServicePort, "bfl-service-port", "", 443, "Bfl ingress ssl port")
	pflag.UintVarP(&periodSeconds, "period-seconds", "", 15, "Period seconds for watch users")

	pflag.Parse()

	klog.InfoS("l4-bfl-proxy flags:",
		"verbose", verbose,
		"enableNginx", enableNginx,
		"inContainer", inContainer,
		"kubeconfig", os.Getenv("KUBECONFIG"),
		"userNamespacePrefix", userNamespacePrefix,
		"nginx-workers", workerProcesses,
		"sslServerPort", sslServerPort,
		"bflServicePort", bflServicePort,
		"periodSeconds", periodSeconds,
	)
	return nil
}

func messageWithError(msg string, err error) error {
	return fmt.Errorf("%s: %v", msg, err)
}

func main() {
	// flags
	err := flags()
	if err != nil {
		klog.Error(err)
		return
	}

	s := &Server{}

	klog.Info("init server")
	if err = s.init(); err != nil {
		klog.Error(err)
		return
	}

	klog.Info("waiting for nginx process is running")
	if !s.waitForNgxStartup() {
		klog.Error("nginx process is still not running yet")
		return
	}

	klog.Info("render /etc/nginx/nginx.conf")
	if err = s.render(); err != nil {
		klog.Errorf("render nginx err, %v", err)
		return
	}

	klog.Info("watch iam users")
	s.watchUser(signal.StopCh(), 5*time.Second)

	klog.Info("all done")
}
