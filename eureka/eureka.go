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

	"github.com/abetobing/goeureka/utility"
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

func NewEureka(eurekaServerUrl, appname, port, username, password string) *Registry {
	r := new(Registry)
	r.DefaultZone = eurekaServerUrl
	r.Username = username
	r.Password = password
	r.AppName = appname
	r.Port = port
	instanceId, err := uuid.NewUUID()
	if err != nil {
		log.Fatalln(fmt.Errorf("Failed generating instance id to be registered to Eureka. %v", err))
	}
	r.InstanceId = fmt.Sprintf("%s:%v", r.AppName, instanceId)
	return r
}

func (r *Registry) StartHeartbeatDaemon() {
	ticker := time.NewTicker(10 * time.Second)
	quit := make(chan os.Signal, 1)
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
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully registered to Eureka")
		r.Up()
	} else {
		log.Println(fmt.Errorf("Registration FAILED with status %v. %v", resp.Status, err))
	}
}

func (r *Registry) Up() {
	requestBody := r.buildBody("UP")
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully update status 'UP' to Eureka")
		r.StartHeartbeatDaemon()
	} else {
		log.Println(fmt.Errorf("Registration FAILED with status %v. %v", resp.Status, err))
	}
}

func (r *Registry) SendHeartbeat() {
	// http://admin:admin@localhost:8761/eureka/apps/MY_AWSOME_GO_MS/localhost
	url := fmt.Sprintf("%s/apps/%s/%s", r.DefaultZone, r.AppName, r.InstanceId)

	resp, err := r.putRequest(url)

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Heartbeat to Eureka [OK]")
	} else {
		log.Println(fmt.Errorf("Heartbeat to Eureka [FAILED] with status %v. %v", resp.Status, err))
	}

}

func (r *Registry) Down() {
	requestBody := r.buildBody("DOWN")
	json, err := json.Marshal(requestBody)
	if err != nil {
		log.Println(fmt.Errorf("Cannot marshal instance body. %v", err))
	}

	payload := strings.NewReader(string(json))
	url := fmt.Sprintf("%s/apps/%s", r.DefaultZone, r.AppName)

	resp, err := r.postRequest(url, payload)

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		log.Println("Successfully update status 'DOWN' to Eureka")
		r.StartHeartbeatDaemon()
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

	portInfo := PortInfo{r.Port, "true"}
	dataCenterInfo := DataCenterInfo{"com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo", "MyOwn"}

	return &RequestBody{
		Instance: InstanceDetails{
			HostName: hostname,
			App:      r.AppName,
			// VipAddress       :
			// SecureVipAddress :
			IpAddr:     ipAddr,
			InstanceId: r.InstanceId,
			Status:     state,
			Port:       portInfo,
			// SecurePort       :
			// HealthCheckUrl   : string         `json:"healthCheckUrl"`,
			// StatusPageUrl    : string         `json:"statusPageUrl"`,
			// HomePageUrl      : string         `json:"homePageUrl"`,
			DataCenterInfo: dataCenterInfo,
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
