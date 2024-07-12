package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
)

type SealedSecretArgocdRepoCredentialsTemplateValues struct {
	Name     string
	Password string
	Type     string
	URL      string
	Username string
}

func createSealedSecretsRelatedFiles(clusterDir string) {
	sealedSecretDir := fmt.Sprintf("%s/sealed-secrets/argo-cd", clusterDir)
	if err := os.MkdirAll(sealedSecretDir, os.ModePerm); err != nil {
		log.Fatalf("❌ Failed creating %s in kubeaid-config repo : %v", sealedSecretDir, err)
	}

	sealedSecretArgocdRepoCredentialsFilePath := fmt.Sprintf("%s/sealed-secrets/argo-cd/kubeaid-config.yaml", clusterDir)
	sealedSecretArgocdRepoCredentialsFile, err := os.Create(sealedSecretArgocdRepoCredentialsFilePath)
	if err != nil {
		log.Fatalf("❌ Failed creating Sealed Secrets ArgoCD repo credentials file at %s : %v", sealedSecretArgocdRepoCredentialsFilePath, err)
	}
	sealedSecretArgocdRepoCredentialsTemplate, err := template.ParseFiles("k8s/cluster/sealed-secrets/argo-cd/kubeaid-config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed parsing Sealed Secrets ArgoCD repo credentials template file at %s : %v", sealedSecretArgocdRepoCredentialsFilePath, err)
	}
	if err = sealedSecretArgocdRepoCredentialsTemplate.Execute(sealedSecretArgocdRepoCredentialsFile, SealedSecretArgocdRepoCredentialsTemplateValues{
		Name: encodeStringToBase64(config.ArgoCD.RepoName),
		URL:  encodeStringToBase64(config.KubeaidConfigRepoURL),
		Type: encodeStringToBase64(config.ArgoCD.RepoType),

		Username: encodeStringToBase64(config.ArgoCD.RepoUsername),
		Password: encodeStringToBase64(config.ArgoCD.RepoAuthToken),
	}); err != nil {
		log.Fatalf("❌ Failed exeuting Sealed Secrets ArgoCD repo credentials template : %v", err)
	}

	kubesealCmd :=
		parseCommand(fmt.Sprintf(`
			kubeseal \
				--kubeconfig %s \
				--controller-name sealed-secrets --controller-namespace kube-system \
				--secret-file %s --sealed-secret-file %s
		`, config.ManagementClusterKubeconfig, sealedSecretArgocdRepoCredentialsFilePath, sealedSecretArgocdRepoCredentialsFilePath))
	log.Printf("Executing kubeseal command : %v", kubesealCmd)
	output, err := kubesealCmd.CombinedOutput()
	if err != nil {
		log.Fatal("❌ Failed generating Sealed Secret from Kubernetes Secret using kubeseal", err)
	}
	log.Print(string(output))

	log.Printf("✅ Created Sealed Secrets ArgoCD repo credentials file at %s", sealedSecretArgocdRepoCredentialsFilePath)
}
