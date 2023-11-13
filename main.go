package main

import (
	"context"
	"errors"
	"flag"
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

var (
	port     int
	filePath string
	address  string
)

func main() {
	flag.IntVar(&port, "p", 5000, "port to listen on")
	flag.StringVar(&address, "a", "0.0.0.0", "address to listen on")
	flag.StringVar(&filePath, "c", "config.yaml", "configuration yaml file")
	flag.Parse()

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
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", address, port), nil)

	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
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
		log.Fatalf("Error setting power on for MAC Address %s, Port Index %s: %v", macAddress, portIdx, err)
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
		log.Fatalf("Error setting power on for MAC Address %s, Port Index %s: %v", macAddress, portIdx, err)
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

	log.Printf("Reading config file %s", filePath)

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return config, fmt.Errorf("File %s does not exist: %v", filePath, err)
	}

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
