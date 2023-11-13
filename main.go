package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/paultyng/go-unifi/unifi"
	"gopkg.in/yaml.v2"
)

// Config represents the structure of your YAML file
type Config struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	APIEndpoint string `yaml:"api_endpoint"`
}

var client *lazyClient = (*lazyClient)(nil)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <path-to-yaml-file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	config, err := readConfig(filePath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	client = &lazyClient{
		user:     config.Username,
		pass:     config.Password,
		baseURL:  config.APIEndpoint,
		insecure: true,
	}

	r := mux.NewRouter()

	r.HandleFunc("/maaspower/{mac_address}/{port_idx}/on", PowerOnHandler).Methods("GET")
	r.HandleFunc("/maaspower/{mac_address}/{port_idx}/off", PowerOffHandler).Methods("GET")
	r.HandleFunc("/maaspower/{mac_address}/{port_idx}/query", QueryHandler).Methods("GET")

	http.Handle("/", r)

	fmt.Println("Server is running on http://0.0.0.0:5000")
	http.ListenAndServe("0.0.0.0:5000", nil)
}

func getPort(ctx context.Context, macAddress string, portIdx string) (deviceId string, port unifi.DevicePortOverrides, err error) {
	deviceId = ""

	p, err := strconv.Atoi(portIdx)
	if err != nil {
		err = fmt.Errorf("Error getting integer value from port %s: %v", portIdx, err)
		return
	}

	i := p - 1

	dev, err := client.GetDeviceByMAC(ctx, "default", macAddress)
	if err != nil {
		err = fmt.Errorf("Error getting device by MAC Address %s: %v", macAddress, err)
		return
	}

	deviceId = dev.ID

	if dev.PortOverrides[i].PortIDX != p {
		err = fmt.Errorf("Error getting port index %s for MAC Address %s", portIdx, macAddress)
		return
	}

	port = dev.PortOverrides[i]

	return
}

func setPortPower(ctx context.Context, macAddress string, portIdx string, power bool) error {
	devId, port, err := getPort(ctx, macAddress, portIdx)
	if err != nil {
		return err
	}

	if power {
		if port.PoeMode == "auto" {
			return nil
		}
		port.PoeMode = "auto"
	} else {
		if port.PoeMode == "off" {
			return nil
		}
		port.PoeMode = "off"
	}

	_, err = client.UpdateDevice(ctx, "default", &unifi.Device{
		ID:            devId,
		PortOverrides: []unifi.DevicePortOverrides{port},
	})

	if err != nil {
		return fmt.Errorf("Error updating device: %v", err)
	}

	return nil
}

func PowerOnHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macAddress := vars["mac_address"]
	portIdx := vars["port_idx"]

	err := setPortPower(r.Context(), macAddress, portIdx, true)
	if err != nil {
		fmt.Fprintf(w, "Error setting power on for MAC Address %s, Port Index %s: %v", macAddress, portIdx, err)
		return
	}
}

func PowerOffHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macAddress := vars["mac_address"]
	portIdx := vars["port_idx"]

	err := setPortPower(r.Context(), macAddress, portIdx, false)
	if err != nil {
		fmt.Fprintf(w, "Error setting power on for MAC Address %s, Port Index %s: %v", macAddress, portIdx, err)
		return
	}
}

func QueryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macAddress := vars["mac_address"]
	portIdx := vars["port_idx"]

	_, port, err := getPort(r.Context(), macAddress, portIdx)
	if err != nil {
		fmt.Fprintf(w, "Error setting power on for MAC Address %s, Port Index %s: %v", macAddress, portIdx, err)
		return
	}

	mode := port.PoeMode

	if mode == "auto" {
		fmt.Fprintf(w, "status : on")
		return
	} else if mode == "off" {
		fmt.Fprint(w, "status : stopped")
		return
	}

	fmt.Fprintf(w, "Query request for MAC Address %s, Port Index %s", macAddress, portIdx)
}

func readConfig(filePath string) (Config, error) {
	var config Config

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
