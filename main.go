package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

var (
	port = flag.Uint("port", 80, "http port")
	consulDnsAddr = flag.String("consulDnsAddr", "", "consul DNS server address. if empty, uses the default system DNS resolver")
)

func main() {
	flag.Parse()

	err := runServer()
	if err != nil {
		panic(err)
	}
}

func runServer() error {
	s := NewServer(*consulDnsAddr)

	http.Handle("/", s)
	return http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}

// Server implements a http.Handler to serve HTTP requests
// with a redirect to the correct port of the Consul service
type Server struct {
	resolver *net.Resolver
}

func NewServer(consulDnsAddr string) *Server {
	if consulDnsAddr == "" {
		return &Server{
			resolver: net.DefaultResolver,
		}
	}

	return &Server{
		resolver: &net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialer := &net.Dialer{}
				return dialer.DialContext(ctx, network, consulDnsAddr)
			},
		},
	}
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
		%s port %d
	</a>
</li>
		`, u, strings.Join(option.Targets, ", "), option.Port)))
	}

	_, _ = res.Write([]byte(fmt.Sprintf("</ul>")))

}

// RedirectOption corresponds to a Consul service+port pair which can be redirected to
type RedirectOption struct {
	Hostname string
	Targets []string
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

	svcName := strings.TrimSuffix(hostname, ".service.consul")
	svcType := "tcp"

	if strings.Contains(svcName, ".") {
		parts := strings.SplitN(svcName, ".", 2)
		svcType = parts[0]
		svcName = parts[1]
	}

	_, addrs, err := s.resolver.LookupSRV(ctx, svcName, svcType, "service.consul")

	if err != nil {
		return options, err
	}

	for _, srv := range addrs {
		targets, err := s.queryConsulTargets(ctx, srv.Target)
		if err != nil {
			return options, err
		}

		log.Printf("%s port %d\n", targets, srv.Port)
		options = append(options, RedirectOption{
			Hostname: hostname,
			Targets: targets,
			Port: srv.Port,
		})
	}

	// sort lowest -> highest port number
	sort.Slice(options, func(i, j int) bool {
		return options[i].Port < options[j].Port
	})

	return options, nil
}

func (s *Server) queryConsulTargets(ctx context.Context, target string) ([]string, error) {
	var targets []string

	// query for IP address from the consul resolver
	addrs, err := s.resolver.LookupIPAddr(ctx, target)
	if err != nil {
		return targets, err
	}

	for _, ip := range addrs {
		// query for reverse dns from the system dns resolver,
		// not the consul resolver
		rev, err := net.LookupAddr(ip.String())
		if err != nil {
			return targets, err
		}
		if len(rev) == 0 {
			continue
		}

		targets = append(targets, strings.TrimSuffix(rev[0], "."))
	}

	return targets, nil
}

func getHostname(req *http.Request) string {
	return strings.SplitN(req.Host, ":", 2)[0]
}