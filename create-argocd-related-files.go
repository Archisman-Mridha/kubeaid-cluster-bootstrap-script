package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func createArgoCDRelatedFiles(clusterDir string, defaultBranchName string, gitAuthMethod transport.AuthMethod) {
	argocdAppsDir := fmt.Sprintf("%s/argocd-apps/templates", clusterDir)
	if err := os.MkdirAll(argocdAppsDir, os.ModePerm); err != nil {
		log.Fatalf("Failed creating dir %s in cluster-dir : %v", argocdAppsDir, err)
	}

	templatesPath := "k8s/cluster/argocd-apps/templates/*"
	templates, err := template.ParseGlob(templatesPath)
	if err != nil {
		log.Fatalf("Failed parsing templates at %s : %v", templatesPath, err)
	}

	for _, argocdAppName := range defaultArgocdApps {
		argocdAppFilePath := fmt.Sprintf("%s/%v.yaml", argocdAppsDir, argocdAppName)
		argocdAppFile, err := os.Create(argocdAppFilePath)
		if err != nil {
			log.Fatalf("Failed creating ArgoCD app file at %s : %v", argocdAppFilePath, err)
		}
		argocdAppTemplateName := fmt.Sprintf("%s.yaml", argocdAppName)
		err = templates.ExecuteTemplate(argocdAppFile, argocdAppTemplateName, ArgocdAppTemplateValues{
			ClusterName:       config.ClusterName,
			KubeAidRepo:       config.KubeaidRepoURL,
			KubeAidConfigRepo: config.KubeaidConfigRepoURL,
			Branch:            defaultBranchName,
		})
		if err != nil {
			log.Fatalf("Failed applying argocd-app template %s to file %s", argocdAppTemplateName, argocdAppFilePath)
		}

		switch argocdAppName {
		case "root":
			log.Info("✅ Generated file for 'root' ArgoCD app")
			continue

		case "kube-prometheus":
			kubePrometheusDir := fmt.Sprintf("%s/kube-prometheus", clusterDir)
			if err := os.MkdirAll(kubePrometheusDir, os.ModePerm); err != nil {
				log.Fatalf("Failed creating %s in kubeaid-config repo : %v", kubePrometheusDir, err)
			}

			// Create the jsonnet file.
			jsonnetFileName := fmt.Sprintf("%s/%s-vars.jsonnet", clusterDir, config.ClusterName)
			jsonnetFile, err := os.Create(jsonnetFileName)
			if err != nil {
				log.Fatalf("Failed creating jsonnet file %s : %v", jsonnetFileName, err)
			}
			jsonnetTemplate, err := template.ParseFiles("k8s/cluster/cluster.jsonnet")
			if err != nil {
				log.Fatal("Failed parsing jsonnet template")
			}
			if err = jsonnetTemplate.Execute(jsonnetFile, JsonnetFileTemplateValues{
				KubePrometheusVersion: config.KubePrometheusVersion,
				GrafanaURL:            config.GrafanaURL,
				ConnectObmondo:        config.ConnectObmondo,
			}); err != nil {
				log.Fatalf("Failed executing jsonnet template against the jsonnet file : %v", err)
			}

			// Clone kubeaid repo.
			kubeaidRepoDir := tempDirPath + "/kubeaid"
			gitCloneRepo(config.KubeaidRepoURL, kubeaidRepoDir, gitAuthMethod)

			// Run the kube-prometheus build script.
			log.Debugf("Running kube-prometheus build script....")
			kubePrometheusBuildScriptPath := fmt.Sprintf("%s/build/kube-prometheus/build.sh", kubeaidRepoDir)
			kubePrometheusBuildCmd := exec.Command(kubePrometheusBuildScriptPath, clusterDir)
			log.Debugf("Executing command : %s", kubePrometheusBuildCmd)
			output, err := kubePrometheusBuildCmd.CombinedOutput()
			if err != nil {
				log.Fatalf("Failed executing kube-prometheus build script : %v", err)
			}
			log.Infof("kube-prometheus build script execution output : %s\n", string(output))

			log.Info("✅ Generated files for 'kube-prometheus' ArgoCD app and ran kube-prometheus build script")

		default:
			argocdAppValuesTemplateFilePath := fmt.Sprintf("k8s/cluster/argocd-apps/values-%s.yaml", argocdAppName)
			argocdAppValuesFilePath := fmt.Sprintf("%s/argocd-apps/values-%s.yaml", clusterDir, argocdAppName)
			if err = copyFile(argocdAppValuesTemplateFilePath, argocdAppValuesFilePath); err != nil {
				log.Fatalf("Failed copying argocd-app values file from %s to %s : %v", argocdAppValuesTemplateFilePath, argocdAppValuesFilePath, err)
			}
			log.Infof("✅ Generated files for %s ArgoCD app", argocdAppName)
		}
	}

	argocdAppsChartTemplateFilePath := "k8s/cluster/argocd-apps/Chart.yaml"
	argocdAppsChartFilePath := fmt.Sprintf("%s/argocd-apps/Chart.yaml", clusterDir)
	if err = copyFile(argocdAppsChartTemplateFilePath, argocdAppsChartFilePath); err != nil {
		log.Fatalf("Failed copying argocd-apps Chart.yaml file from %s to %s : %v", argocdAppsChartTemplateFilePath, argocdAppsChartFilePath, err)
	}
}
