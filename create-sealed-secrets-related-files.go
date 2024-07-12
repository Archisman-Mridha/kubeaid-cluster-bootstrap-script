package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

func createSealedSecretsRelatedFiles(clusterDir string) {
	sealedSecretDir := fmt.Sprintf("%s/sealed-secrets/argo-cd", clusterDir)
	if err := os.MkdirAll(sealedSecretDir, os.ModePerm); err != nil {
		log.Fatalf("Failed creating %s in kubeaid-config repo : %v", sealedSecretDir, err)
	}

	sealedSecretArgocdRepoCredentialsFilePath := fmt.Sprintf("%s/sealed-secrets/argo-cd/kubeaid-config.yaml", clusterDir)
	sealedSecretArgocdRepoCredentialsFile, err := os.Create(sealedSecretArgocdRepoCredentialsFilePath)
	if err != nil {
		log.Fatalf("Failed creating Sealed Secrets ArgoCD repo credentials file at %s : %v", sealedSecretArgocdRepoCredentialsFilePath, err)
	}
	sealedSecretArgocdRepoCredentialsTemplate, err := template.ParseFiles("k8s/cluster/sealed-secrets/argo-cd/kubeaid-config.yaml")
	if err != nil {
		log.Fatalf("Failed parsing Sealed Secrets ArgoCD repo credentials template file at %s : %v", sealedSecretArgocdRepoCredentialsFilePath, err)
	}
	if err = sealedSecretArgocdRepoCredentialsTemplate.Execute(sealedSecretArgocdRepoCredentialsFile, SealedSecretArgocdRepoCredentialsTemplateValues{
		Name: encodeStringToBase64(config.ArgoCD.RepoName),
		URL:  encodeStringToBase64(config.KubeaidConfigRepoURL),
		Type: encodeStringToBase64(config.ArgoCD.RepoType),

		Username: encodeStringToBase64(config.ArgoCD.RepoUsername),
		Password: encodeStringToBase64(config.ArgoCD.RepoAuthToken),
	}); err != nil {
		log.Fatalf("Failed exeuting Sealed Secrets ArgoCD repo credentials template : %v", err)
	}

	kubesealCmd := exec.Command(
		"kubeseal",
		"--controller-name", "sealed-secrets",
		"--controller-namespace", "kube-system",
		"--secret-file", sealedSecretArgocdRepoCredentialsFilePath,
		"--sealed-secret-file", sealedSecretArgocdRepoCredentialsFilePath,
	)
	log.Debugf("Executing kubeseal command : %v", kubesealCmd)
	sealedSecretOutput, err := kubesealCmd.CombinedOutput()
	if err != nil {
		log.Fatal("Failed generating Sealed Secret from Kubernetes Secret using kubeseal", err)
	}
	log.Debugf("Creating Sealed Secrets ArgoCD repo credentials file | Kubeseal output : %s", string(sealedSecretOutput))

	log.Infof("✅ Created Sealed Secrets ArgoCD repo credentials file at %s", sealedSecretArgocdRepoCredentialsFilePath)
}

// kubeseal \
// 	--controller-name sealed-secrets \
// 	--controller-namespace kube-system \
// 	--kubeconfig ~/.kube/config \
// 	--secret-file /tmp/kubeaid-bootstrap-script-1720684065679112417/kubeaid-config/k8s/kubeaid-cluster-bootstrap-script-demo/sealed-secrets/argo-cd/kubeaid-config.yaml \
// 	--sealed-secret-file /tmp/kubeaid-bootstrap-script-1720684065679112417/kubeaid-config/k8s/kubeaid-cluster-bootstrap-script-demo/sealed-secrets/argo-cd/kubeaid-config.yaml
