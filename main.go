package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/consul/api"
)

var (
	port = flag.Uint("port", 80, "http port")
)

func main() {
	flag.Parse()

	err := runServer()
	if err != nil {
		panic(err)
	}
}

func runServer() error {
	s, err := NewServer()
	if err != nil {
		return err
	}

	http.Handle("/", s)
	return http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}

// Server implements a http.Handler to serve HTTP requests
// with a redirect to the correct port of the Consul service
type Server struct {
	consul *api.Client
}

func NewServer() (*Server, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &Server{
		consul: client,
	}, nil
}

// ServeHTTP redirects to the requested port, or provides a list of
// which ports exist to redirect to.
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request)  {
	hostname := getHostname(req)
	log.Printf("request: %s%s\n", req.Host, req.URL.Path)

	if !strings.HasSuffix(hostname, ".service.consul") {
		res.WriteHeader(404)
		_, _ = res.Write([]byte(fmt.Sprintf("unable to parse hostname as .service.consul address: %s", hostname)))
		return
	}

	result, err := s.queryConsulSRV(context.Background(), hostname)
	if err != nil {
		log.Printf("error querying %s: %#v", hostname, err)
		return
	}

	if len(result) == 1 {
		u, err := result[0].BuildURL(hostname, req.URL)
		if err != nil {
			log.Printf("error building URL for %s: %#v", hostname, err)
			return
		}

		log.Printf("redirecting to %s", u.String())

		http.Redirect(res, req, u.String(), 307)
		return
	}

	if len(result) == 0 {
		res.WriteHeader(404)
		_, _ = res.Write([]byte(fmt.Sprintf("Couldn't process hostname %s", hostname)))
		return
	}

	res.WriteHeader(200)
	_, _ = res.Write([]byte(fmt.Sprintf("<ul>")))
	for _, option := range result {
		u, err := option.BuildURL(hostname, req.URL)
		if err != nil {
			log.Printf("error building URL for %s: %#v", hostname, err)
			return
		}

		_, _ = res.Write([]byte(fmt.Sprintf(`
<li>
	<a href="%s">
		%s port %d (%s)
	</a>
</li>
		`, u, option.Hostname, option.Port, strings.Join(option.Tags, ", "))))
	}

	_, _ = res.Write([]byte(fmt.Sprintf("</ul>")))

}

// RedirectOption corresponds to a Consul service+port pair which can be redirected to
type RedirectOption struct {
	Hostname string
	Tags []string
	Port uint16
}

// BuildURL replaces the port in the given URL
func (r *RedirectOption) BuildURL(hostname string, origUrl *url.URL) (*url.URL, error) {
	u, err := url.Parse(origUrl.String())
	if err != nil {
		return nil, err
	}

	u.Scheme = "http"
	u.Host = fmt.Sprintf("%s:%d", hostname, r.Port)

	return u, nil
}

func (s *Server) queryConsulSRV(ctx context.Context, hostname string) ([]RedirectOption, error) {
	var options []RedirectOption

	svcName, svcType := parseConsulAddress(hostname)

	services, _, err := s.consul.Catalog().Service(svcName, svcType, &api.QueryOptions{})
	if err != nil {
		return options, err
	}

	for _, svc := range services {
		log.Printf("%s port %d: %#v\n", svc.Address, svc.ServicePort, *svc)

		options = append(options, RedirectOption{
			Hostname: svc.Node,
			Tags: svc.ServiceTags,
			Port: uint16(svc.ServicePort),
		})
	}

	// sort lowest -> highest port number for each hostname
	sort.Slice(options, func(i, j int) bool {
		return options[i].Hostname < options[j].Hostname && options[i].Port < options[j].Port
	})

	return options, nil
}

func parseConsulAddress(hostname string) (svcName, svcType string) {
	svcName = strings.TrimSuffix(hostname, ".service.consul")
	svcType = ""

	if strings.Contains(svcName, ".") {
		parts := strings.SplitN(svcName, ".", 2)
		svcType = parts[0]
		svcName = parts[1]
	}

	return svcName, svcType
}

func getHostname(req *http.Request) string {
	return strings.SplitN(req.Host, ":", 2)[0]
}