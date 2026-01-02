package main

import (
	"context"
	"encoding/json"
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

type Config struct {
	Ingresses []IngressConfig `yaml:"ingresses"`
}

type IngressConfig struct {
	Annotations map[string]string `yaml:"annotations"`
}

type App struct {
	Title  string   `json:"title"`
	Icon   string   `json:"icon"`
	URL    string   `json:"url"`
	Groups []string `json:"groups"`
}

var demoMode bool

func main() {
	demoMode = os.Getenv("DEMO_MODE") == "true"

	http.HandleFunc("/api/apps", handleApps)
	log.Println("Backend starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userGroups := getUserGroups(r)

	var apps []App
	var err error

	if demoMode {
		apps, err = getDemoApps()
	} else {
		apps, err = getK8sApps()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filtered := filterAppsByGroups(apps, userGroups)
	json.NewEncoder(w).Encode(filtered)
}

func getUserGroups(r *http.Request) []string {
	groupsHeader := r.Header.Get("X-Auth-Groups")
	if groupsHeader == "" {
		return []string{}
	}
	return strings.Split(groupsHeader, ",")
}

func getDemoApps() ([]App, error) {
	data, err := os.ReadFile("/etc/dashboard/config.yaml")
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var apps []App
	for _, ing := range config.Ingresses {
		if ing.Annotations["dashboard.home/enabled"] != "true" {
			continue
		}

		app := App{
			Title: ing.Annotations["dashboard.home/title"],
			Icon:  ing.Annotations["dashboard.home/icon"],
			URL:   "https://example.com",
		}

		if groups := ing.Annotations["dashboard.home/groups"]; groups != "" {
			app.Groups = strings.Split(groups, ",")
		}

		apps = append(apps, app)
	}

	return apps, nil
}

func getK8sApps() ([]App, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	ingresses, err := clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var apps []App
	for _, ing := range ingresses.Items {
		if ing.Annotations["dashboard.home/enabled"] != "true" {
			continue
		}

		app := App{
			Title: ing.Annotations["dashboard.home/title"],
			Icon:  ing.Annotations["dashboard.home/icon"],
			URL:   getIngressURL(&ing),
		}

		if groups := ing.Annotations["dashboard.home/groups"]; groups != "" {
			app.Groups = strings.Split(groups, ",")
		}

		apps = append(apps, app)
	}

	return apps, nil
}

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
				if strings.TrimSpace(appGroup) == strings.TrimSpace(userGroup) {
					filtered = append(filtered, app)
					goto next
				}
			}
		}
	next:
	}

	return filtered
}
