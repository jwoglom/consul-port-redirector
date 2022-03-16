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
	port             = flag.Uint("port", 80, "http port")
	nomadUIHostname  = flag.String("nomadUIHostname", "", "the hostname to link to for viewing the Nomad UI")
	consulUIHostname = flag.String("consulUIHostname", "", "the hostname to link to for viewing the Consul UI")
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
	log.Printf("listening on port :%d", *port)
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
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	hostname := getHostname(req)
	log.Printf("request: %s%s", req.Host, req.URL.Path)

	if !strings.HasSuffix(hostname, ".service.consul") {
		log.Printf("unable to parse hostname as .service.consul address: %s", hostname)

		res.Header().Set("Content-Type", "text/html")
		_, _ = res.Write([]byte(fmt.Sprintf(`
<p>No .service.consul address found for <code>%s</code></p>
		`, hostname)))

		s.printQuickLinks(res, hostname)
		return
	}

	result, err := s.queryConsulForHostname(context.Background(), hostname)
	if err != nil {
		log.Printf("error querying Consul for %s: %#v", hostname, err)
		http.Error(res, fmt.Sprintf("error querying Consul for %s: %#v", hostname, err), http.StatusInternalServerError)
		return
	}

	if len(result) == 1 {
		u, err := result[0].BuildURL(hostname, req.URL)
		if err != nil {
			log.Printf("error building URL for %s: %#v", hostname, err)
			http.Error(res, fmt.Sprintf("<p>error building URL for %s: %#v</p>", hostname, err), http.StatusInternalServerError)
			return
		}

		log.Printf("redirecting to %s", u.String())

		http.Redirect(res, req, u.String(), http.StatusTemporaryRedirect)
		return
	}

	if len(result) == 0 {
		res.Header().Set("Content-Type", "text/html")
		http.Error(res, fmt.Sprintf("<p>No results found for hostname %s</p>", hostname), 404)
		s.printQuickLinks(res, hostname)
		return
	}

	res.Header().Set("Content-Type", "text/html")

	_, _ = res.Write([]byte(fmt.Sprintf(`
<p>Service ports found for <code>%s</code>:</p><ul>
	`, hostname)))

	for _, option := range result {
		u, err := option.BuildURL(hostname, req.URL)
		if err != nil {
			log.Printf("error building URL for %s: %#v", hostname, err)
			return
		}

		tags := strings.Join(option.Tags, ", ")
		if len(tags) > 0 {
			tags = " (" + tags + ")"
		}
		_, _ = res.Write([]byte(fmt.Sprintf(`
<li>
	<a href="%s">
		%s port %d%s
	</a>
</li>
		`, u, option.Hostname, option.Port, tags)))
	}

	_, _ = res.Write([]byte("</ul>"))

}

func (s *Server) printQuickLinks(res http.ResponseWriter, hostname string) {
	nomadHost := hostname
	consulHost := hostname

	if len(*nomadUIHostname) > 0 {
		nomadHost = *nomadUIHostname
	}
	if len(*consulUIHostname) > 0 {
		consulHost = *consulUIHostname
	}

	_, _ = res.Write([]byte(fmt.Sprintf(`
<p>Quick links:</p>
<ul>
<li><a href="http://%s:4646/ui/">Nomad UI</a></li>
<li><a href="http://%s:8500/ui/">Consul UI</a></li>
</ul>
	`, nomadHost, consulHost)))
}

// RedirectOption corresponds to a Consul service+port pair which can be redirected to
type RedirectOption struct {
	Hostname string
	Tags     []string
	Port     uint16
}

// BuildURL replaces the port in the given URL
func (r *RedirectOption) BuildURL(hostname string, origUrl *url.URL) (*url.URL, error) {
	u, err := url.Parse(origUrl.String())
	if err != nil {
		return nil, err
	}

	u.Scheme = r.guessScheme()
	u.Host = fmt.Sprintf("%s:%d", hostname, r.Port)

	return u, nil
}

func (r *RedirectOption) guessScheme() string {
	for _, tag := range r.Tags {
		switch strings.ToLower(tag) {
		case "http":
			return "http"
		case "https":
			return "https"
		}
	}
	return "http"
}

func (s *Server) queryConsulForHostname(ctx context.Context, hostname string) ([]RedirectOption, error) {
	var options []RedirectOption

	svcName, svcType := parseConsulAddress(hostname)

	services, _, err := s.consul.Catalog().Service(svcName, svcType, &api.QueryOptions{})
	if err != nil {
		return options, err
	}

	for _, svc := range services {
		log.Printf("%s port %d: %#v", svc.Address, svc.ServicePort, *svc)

		options = append(options, RedirectOption{
			Hostname: svc.Node,
			Tags:     svc.ServiceTags,
			Port:     uint16(svc.ServicePort),
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
