package eureka

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/abetobing/go-eureka/utility"
	"github.com/google/uuid"
)

type RequestBody struct {
	Instance InstanceDetails `json:"instance"`
}
type InstanceDetails struct {
	HostName         string         `json:"hostName"`
	App              string         `json:"app"`
	VipAddress       string         `json:"vipAddress"`
	SecureVipAddress string         `json:"secureVipAddress"`
	InstanceId       string         `json:"instanceId"`
	IpAddr           string         `json:"ipAddr"`
	Status           string         `json:"status"`
	Port             PortInfo       `json:"port"`
	SecurePort       PortInfo       `json:"securePort"`
	HealthCheckUrl   string         `json:"healthCheckUrl"`
	StatusPageUrl    string         `json:"statusPageUrl"`
	HomePageUrl      string         `json:"homePageUrl"`
	DataCenterInfo   DataCenterInfo `json:"dataCenterInfo"`
}
type PortInfo struct {
	Port    string `json:"$"`
	Enabled string `json:"@enabled"`
}

type DataCenterInfo struct {
	Class string `json:"@class"`
	Name  string `json:"name"`
}
type Registry struct {
	AppName     string
	DefaultZone string
	Port        string
	Username    string
	Password    string
	InstanceId  string
}

type InitOptions struct {
	Port     string
	Username string
	Password string
	Verbose  bool
}

var quit chan os.Signal = make(chan os.Signal, 1)
var rto chan bool = make(chan bool)

const (
	RETRY_SECONDS = time.Second * 10
)

var opt *InitOptions = &InitOptions{
	Port:     "8080",
	Username: "",
	Password: "",
	Verbose:  false,
}

func NewEureka(eurekaServerUrl, appname string, initOpt *InitOptions) *Registry {
	opt = initOpt
	r := new(Registry)
	r.DefaultZone = eurekaServerUrl
	if opt != nil {
		if opt.Port != "" {
			r.Port = opt.Port
		}
		if opt.Username != "" {
			r.Username = opt.Username
		}
		if opt.Password != "" {
			r.Password = opt.Password
		}
	}
	r.AppName = appname
	r.Port = opt.Port
	instanceId, err := uuid.NewUUID()
	if err != nil {
		log.Fatalln(fmt.Errorf("Failed generating instance id to be registered to Eureka. %v", err))
	}
	r.InstanceId = fmt.Sprintf("%s:%v", r.AppName, instanceId)
	return r
}

func (r *Registry) StartHeartbeatDaemon() {
	ticker := time.NewTicker(10 * time.Second)
	// quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	go func() {
		for {
			select {
			case <-ticker.C:
				r.SendHeartbeat()
			case <-quit:
				ticker.Stop()
				r.Down()
				log.Println("Terminating in 3 seconds")
				time.Sleep(3 * time.Second)
				os.Exit(0)
				return
			}
		}
	}()
}

func (r *Registry) Register() {
	requestBody := r.buildBody("STARTING")
	log.Printf("Registering to %s to [%s:%s]\n", r.AppName, r.DefaultZone, r.Port)
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
		return
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)

	if err != nil {
		log.Printf("Error registering. %v\n", err)
		time.Sleep(RETRY_SECONDS)
		r.Register()
		return
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully registered to Eureka")
		r.Up()
	} else {
		log.Println(fmt.Errorf("Registration FAILED with status %v. %v", resp.Status, err))
		time.Sleep(RETRY_SECONDS)
		r.Register()
	}
}

func (r *Registry) Up() {
	requestBody := r.buildBody("UP")
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
		return
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)
	if err != nil {
		log.Printf("Error sending UP status. %v\n", err)
		time.Sleep(RETRY_SECONDS)
		r.Register()
		return
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully update status 'UP' to Eureka")
		r.StartHeartbeatDaemon()
	} else {
		log.Println(fmt.Errorf("Registration FAILED with status %v. %v", resp.Status, err))
		time.Sleep(RETRY_SECONDS)
		r.Register()
		r.Register()
	}
}

func (r *Registry) SendHeartbeat() {
	// http://admin:admin@localhost:8761/eureka/apps/MY_AWSOME_GO_MS/localhost
	url := fmt.Sprintf("%s/apps/%s/%s", r.DefaultZone, r.AppName, r.InstanceId)

	resp, err := r.putRequest(url)
	if err != nil {
		log.Println(fmt.Errorf("Can't send heartbeat to eureka. Possibly down, out of reach, network issue."))
		time.Sleep(RETRY_SECONDS)
		r.Register()
		return
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		if opt.Verbose {
			log.Println("Heartbeat to Eureka [OK]")
		}
	} else {
		log.Println(fmt.Errorf("Heartbeat to Eureka [FAILED] with status %v. %v", resp.Status, err))
		time.Sleep(RETRY_SECONDS)
		r.Register()
	}

}

func (r *Registry) Down() {
	requestBody := r.buildBody("DOWN")
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
		return
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)
	if err != nil {
		log.Printf("Error sending DOWN status. %v\n", err)
		return
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully update status 'DOWN' to Eureka")
	} else {
		log.Println(fmt.Errorf("Updating state FAILED with status %v. %v", resp.Status, err))
	}
}

func (r *Registry) buildBody(state string) *RequestBody {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = r.AppName
		log.Println("Can't get hostname form OS, using appname as host name")
	}
	ipAddr, err := utility.ExternalIP()
	if err != nil {
		log.Println("Can't get external IP address. Using 127.0.0.1 as default", err)
		log.Println(fmt.Errorf("Can't get external IP address. Using 127.0.0.1 as default. %v", err))
		ipAddr = "127.0.0.1"
	}
	hostname = ipAddr // force hostname = ipAddr

	portInfo := PortInfo{r.Port, "true"}
	securePortInfo := PortInfo{"443", "false"}
	scheme := "http"
	homePageUrl := fmt.Sprintf("%s://%s:%s/", scheme, ipAddr, r.Port)
	healthCheckUrl := fmt.Sprintf("%shealth", homePageUrl)
	statusPageUrl := fmt.Sprintf("%sinfo", homePageUrl)
	vipAddress := strings.ToLower(r.AppName)
	secureVipAddress := strings.ToLower(r.AppName)
	dataCenterInfo := DataCenterInfo{"com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo", "MyOwn"}

	return &RequestBody{
		Instance: InstanceDetails{
			HostName:         hostname,
			App:              r.AppName,
			VipAddress:       vipAddress,
			SecureVipAddress: secureVipAddress,
			IpAddr:           ipAddr,
			InstanceId:       r.InstanceId,
			Status:           state,
			Port:             portInfo,
			SecurePort:       securePortInfo,
			HomePageUrl:      homePageUrl,
			HealthCheckUrl:   healthCheckUrl,
			StatusPageUrl:    statusPageUrl,
			DataCenterInfo:   dataCenterInfo,
		},
	}
}

func (r *Registry) postRequest(url string, payload io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, payload)
	if err != nil {
		log.Println(fmt.Errorf("Error initiating request. %v", err))
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(r.Username, r.Password)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println(fmt.Errorf("Cannot make POST request to %s. %v", url, err))
		return nil, err
	}

	return resp, nil
}

func (r *Registry) putRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		log.Println(fmt.Errorf("Error initiating request. %v", err))
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(r.Username, r.Password)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println(fmt.Errorf("Cannot make PUT request to %s. %v", url, err))
		return nil, err
	}

	return resp, nil
}
