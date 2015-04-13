package api

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/VividCortex/siesta"

	"github.com/PreetamJinka/cistern/api/context"
	"github.com/PreetamJinka/cistern/device"
	"github.com/PreetamJinka/cistern/state/flows"
	"github.com/PreetamJinka/cistern/state/series"
)

type APIServer struct {
	addr           string
	deviceRegistry *device.Registry
	seriesEngine   *series.Engine
}

func NewAPIServer(address string,
	deviceRegistry *device.Registry,
	seriesEngine *series.Engine) *APIServer {
	return &APIServer{
		addr:           address,
		deviceRegistry: deviceRegistry,
		seriesEngine:   seriesEngine,
	}
}

func (s *APIServer) Run() {
	service := siesta.NewService("/")

	service.AddPre(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
	})

	service.AddPost(func(c siesta.Context, w http.ResponseWriter, r *http.Request, q func()) {
		resp := c.Get(context.Response)
		err, _ := c.Get(context.Error).(string)

		if resp == nil && err == "" {
			return
		}

		enc := json.NewEncoder(w)
		enc.Encode(APIResponse{
			Data:  resp,
			Error: err,
		})
	})

	service.Route("GET", "/", "Default page", func(c siesta.Context, w http.ResponseWriter, r *http.Request) {
		c.Set(context.Response, "Welcome to the Cistern API!")
	})

	service.Route("GET", "/devices", "Lists sources", func(c siesta.Context, w http.ResponseWriter, r *http.Request) {
		type ipHostname struct {
			IP       string `json:"ip"`
			Hostname string `json:"hostname,omitempty"`
		}

		devices := []ipHostname{}

		for _, dev := range s.deviceRegistry.Devices() {
			devices = append(devices, ipHostname{
				IP:       dev.IP().String(),
				Hostname: dev.Hostname(),
			})
		}

		c.Set(context.Response, devices)
	})

	service.Route("GET", "/devices/:device/metrics",
		"Lists metrics for a device",
		func(c siesta.Context, w http.ResponseWriter, r *http.Request) {
			var params siesta.Params
			device := params.String("device", "", "Device name")
			err := params.Parse(r.Form)
			if err != nil {
				c.Set(context.Error, err.Error())
				return
			}

			address := net.ParseIP(*device)
			dev, present := s.deviceRegistry.Lookup(address)
			if !present {
				c.Set(context.Error, "device not found")
				return
			}

			c.Set(context.Response, dev.Metrics())
		})

	service.Route("GET", "/devices/:device/flows",
		"Lists top flows for a device",
		func(c siesta.Context, w http.ResponseWriter, r *http.Request) {
			var params siesta.Params
			device := params.String("device", "", "Device name")
			err := params.Parse(r.Form)
			if err != nil {
				c.Set(context.Error, err.Error())
				return
			}

			address := net.ParseIP(*device)
			dev, present := s.deviceRegistry.Lookup(address)
			if !present {
				c.Set(context.Error, "device not found")
				return
			}

			type flowsResponse struct {
				ByBytes   []flows.Flow `json:"byBytes"`
				ByPackets []flows.Flow `json:"byPackets"`
			}

			topTalkers := dev.TopTalkers()
			if topTalkers == nil {
				c.Set(context.Error, "No active flows")
				return
			}

			resp := flowsResponse{
				ByBytes:   topTalkers.ByBytes(),
				ByPackets: topTalkers.ByPackets(),
			}

			c.Set(context.Response, resp)
		})

	service.Route("OPTIONS", "/series/query",
		"Accepts an OPTIONS request",
		func(w http.ResponseWriter, r *http.Request) {
			// Doesn't do anything
		})

	service.Route("POST", "/series/query",
		"Lists metrics for a device",
		s.querySeriesRoute())

	http.Handle("/", service)

	go http.ListenAndServe(s.addr, nil)
}
