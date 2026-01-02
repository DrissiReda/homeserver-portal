package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//go:embed static/*
var staticFiles embed.FS

type Config struct {
	Groups    string          `yaml:"groups"`
	Ingresses []IngressConfig `yaml:"ingresses"`
}

type IngressConfig struct {
	Annotations map[string]string `yaml:"annotations"`
}

type App struct {
	Title       string   `json:"title"`
	Icon        string   `json:"icon"`
	URL         string   `json:"url"`
	Groups      []string `json:"groups"`
	Description string   `json:"description"`
}

var (
	demoMode   bool
	demoGroups []string
	staticFS   fs.FS
	debugMode  bool
)

func main() {
	demoMode = os.Getenv("DEMO_MODE") == "true"
	logLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	debugMode = logLevel == "DEBUG"

	if demoMode {
		loadDemoGroups()
	}

	log.Printf("Starting portal server (DEMO_MODE=%v DEBUG=%v)", demoMode, debugMode)

	// Initialize static file system
	var err error
	staticFS, err = fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load static files: %v", err)
	}

	// API endpoints
	http.HandleFunc("/api/apps", handleApps)
	http.HandleFunc("/health", handleHealth)

	// Static file handler
	http.HandleFunc("/", serveStatic)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting portal server on :%s (DEMO_MODE=%v)", port, demoMode)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// serveStatic serves static files or returns 404
func serveStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Read file from embedded filesystem
	content, err := fs.ReadFile(staticFS, path)
	if err != nil {
		http.Error(w, "404 - Page Not Found", http.StatusNotFound)
		return
	}

	// Set content type based on file extension
	contentType := getContentType(path)
	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

// getContentType returns the appropriate content type for a file
func getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// handleApps returns filtered apps based on user groups
func handleApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	userGroups := getUserGroups(r)
	log.Printf("Apps request: user_groups=%v remote_addr=%s", userGroups, r.RemoteAddr)

	var apps []App
	var err error

	if demoMode {
		apps, err = getDemoApps()
	} else {
		apps, err = getK8sApps()
	}

	if err != nil {
		log.Printf("ERROR fetching apps: %v", err)
		http.Error(w, `{"error":"failed to fetch apps"}`, http.StatusInternalServerError)
		return
	}

	filtered := filterAppsByGroups(apps, userGroups)
	log.Printf("Apps response: total=%d filtered=%d", len(apps), len(filtered))

	if err := json.NewEncoder(w).Encode(filtered); err != nil {
		log.Printf("ERROR encoding apps response: %v", err)
	}
}

// handleHealth is a liveness/readiness probe endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// getUserGroups extracts user groups from X-Auth-Request-Groups header
func getUserGroups(r *http.Request) []string {
	if debugMode {
		log.Printf("DEBUG: All request headers:")
		for key, values := range r.Header {
			for _, value := range values {
				log.Printf("  %s: %s", key, value)
			}
		}
	}

	if demoMode {
		log.Printf("DEBUG: Using demo mode groups")
		return demoGroups
	}

	groupsHeader := r.Header.Get("X-Forwarded-Groups")
	if debugMode {
		log.Printf("DEBUG: X-Forwarded-Groups header value: %q", groupsHeader)
	}

	if groupsHeader == "" {
		log.Printf("WARNING: No groups found in x-auth-request-groups header")
		return []string{}
	}

	groups := strings.Split(groupsHeader, ",")
	for i := range groups {
		groups[i] = strings.TrimSpace(groups[i])
	}
	
	log.Printf("Parsed groups from header: %v", groups)
	return groups
}

// loadDemoGroups loads group configuration from YAML file for demo mode
func loadDemoGroups() {
	data, err := os.ReadFile("/etc/dashboard/config.yaml")
	if err != nil {
		data, err = os.ReadFile("config.yaml")
		if err != nil {
			log.Printf("WARNING: Failed to load demo groups config: %v", err)
			return
		}
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("WARNING: Failed to parse demo groups: %v", err)
		return
	}

	if config.Groups != "" {
		demoGroups = strings.Split(config.Groups, ",")
		for i := range demoGroups {
			demoGroups[i] = strings.TrimSpace(demoGroups[i])
		}
		log.Printf("Demo mode enabled with groups: %v", demoGroups)
	}
}

// getDemoApps loads apps from local config.yaml for development/testing
func getDemoApps() ([]App, error) {
	data, err := os.ReadFile("/etc/dashboard/config.yaml")
	if err != nil {
		data, err = os.ReadFile("config.yaml")
		if err != nil {
			return nil, err
		}
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	log.Printf("Demo mode: loading %d ingress configs from file", len(config.Ingresses))

	var apps []App
	for _, ing := range config.Ingresses {
		if ing.Annotations["dashboard.home/enabled"] != "true" {
			continue
		}

		app := App{
			Title:       ing.Annotations["dashboard.home/title"],
			Icon:        ing.Annotations["dashboard.home/icon"],
			Description: ing.Annotations["dashboard.home/description"],
			URL:         "https://example.com",
		}

		if groups := ing.Annotations["dashboard.home/groups"]; groups != "" {
			app.Groups = strings.Split(groups, ",")
		}

		apps = append(apps, app)
	}

	log.Printf("Demo mode: %d apps enabled", len(apps))
	return apps, nil
}

// getK8sApps queries Kubernetes API for Ingress resources with dashboard annotations
func getK8sApps() ([]App, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("ERROR: Failed to get in-cluster config: %v", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("ERROR: Failed to create Kubernetes clientset: %v", err)
		return nil, err
	}

	ingresses, err := clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("ERROR: Failed to list ingresses: %v", err)
		return nil, err
	}

	log.Printf("Kubernetes mode: found %d total ingresses", len(ingresses.Items))

	var apps []App
	for _, ing := range ingresses.Items {
		if ing.Annotations["dashboard.home/enabled"] != "true" {
			continue
		}

		app := App{
			Title:       ing.Annotations["dashboard.home/title"],
			Icon:        ing.Annotations["dashboard.home/icon"],
			Description: ing.Annotations["dashboard.home/description"],
			URL:         getIngressURL(&ing),
		}

		if groups := ing.Annotations["dashboard.home/groups"]; groups != "" {
			app.Groups = strings.Split(groups, ",")
		}

		apps = append(apps, app)
		log.Printf("Added app: title=%s namespace=%s groups=%v", app.Title, ing.Namespace, app.Groups)
	}

	log.Printf("Kubernetes mode: %d apps enabled", len(apps))
	return apps, nil
}

// getIngressURL constructs the URL from ingress configuration
func getIngressURL(ing *v1.Ingress) string {
	if len(ing.Spec.Rules) > 0 {
		host := ing.Spec.Rules[0].Host
		if len(ing.Spec.TLS) > 0 {
			return "https://" + host
		}
		return "http://" + host
	}
	return ""
}

// filterAppsByGroups filters apps based on user's group membership
func filterAppsByGroups(apps []App, userGroups []string) []App {
	if len(userGroups) == 0 {
		return apps
	}

	var filtered []App
	for _, app := range apps {
		if len(app.Groups) == 0 {
			filtered = append(filtered, app)
			continue
		}

		for _, appGroup := range app.Groups {
			for _, userGroup := range userGroups {
				if strings.EqualFold(strings.TrimSpace(appGroup), strings.TrimSpace(userGroup)) {
					filtered = append(filtered, app)
					goto next
				}
			}
		}
	next:
	}

	return filtered
}