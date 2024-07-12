package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

type Config struct {
	Git struct {
		Username string `yaml:"username"`

		Password        string `yaml:"password"`
		SSHPrivateKey   string `yaml:"sshPrivateKey"`
		UseSSHAgentAuth bool   `yaml:"useSSHAgentAuth"`
	} `yaml:"git"`

	KubeaidRepoURL       string `yaml:"kubeaidRepoURL"`
	KubeaidConfigRepoURL string `yaml:"kubeaidConfigRepoURL"`

	ClusterName string `yaml:"clusterName"`

	ArgoCD struct {
		RepoName      string `yaml:"repoName"`
		RepoType      string `yaml:"repoType"`
		RepoUsername  string `yaml:"repoUsername"`
		RepoAuthToken string `yaml:"repoAuthToken"`
	} `yaml:"argoCD"`

	KubePrometheusVersion string `yaml:"kubePrometheusVersion"`
	GrafanaURL            string `yaml:"grafanaURL"`
	ConnectObmondo        bool   `yaml:"connectObmondo"`

	ManagementClusterKubeconfig string `yaml:"managementClusterKubeconfig"`
	ManagementClusterKubectx    string `yaml:"managementClusterKubectx"`
}

var (
	currentTime = time.Now().Unix()

	tempDirPath = getTempDirPath()
	repoDir     = tempDirPath + "/kubeaid-config"

	config Config
)

func main() {
	// Delete the temp dir after the script finishes running.
	defer os.RemoveAll(tempDirPath)

	configFile := flag.String("config-file", "", "Path to the YAML config file")
	flag.Parse()

	log.Printf("💫 Running the kubeaid cluster bootstrap script")

	// Ensure CLI tools are installed.
	ensurePrerequisitesInstalled()

	// Parse CLI flags.
	parseConfigFile(configFile)

	// Detect git authentication method.
	gitAuthMethod := getGitAuthMethod()

	// Use specified kube-context.
	log.Printf("⚙️ Setting context to %s in kubeconfig at %s", config.ManagementClusterKubectx, config.ManagementClusterKubectx)
	kubectlConfigCmd := parseCommand(fmt.Sprintf(
		"kubectl config use-context %s --kubeconfig %s",
		config.ManagementClusterKubectx, config.ManagementClusterKubeconfig,
	))
	log.Printf("Executing command : %v", kubectlConfigCmd)
	output, err := kubectlConfigCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("❌ Failed setting context to %s in the kubeconfig at %s", config.ManagementClusterKubectx, config.ManagementClusterKubectx)
	}
	log.Println(string(output))

	// Clone kubeaid-config repo.
	repo := gitCloneRepo(config.KubeaidConfigRepoURL, repoDir, gitAuthMethod)
	repoDefaultBranchName := getDefaultBranchName(repo)

	// Create and checkout to a new branch.
	repoWorktree, err := repo.Worktree()
	if err != nil {
		log.Fatal("❌ Failed getting kubeaid-config repo worktree")
	}
	branch := fmt.Sprintf("kubeaid-%s-%d", config.ClusterName, currentTime)
	createAndCheckoutToBranch(repo, branch, repoWorktree)

	// In the k8s dir, we will create a folder for the cluster. Files related to the cluster, will
	// be generated in this folder.
	clusterDir := fmt.Sprintf("%s/k8s/%s", repoDir, config.ClusterName)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		log.Fatalf("❌ Cluster dir %s already exists", clusterDir)
	} else if err != nil {
		log.Fatalf("Failed determining whether cluster-dir exists or not : %v", err)
	}

	// Generate files for ArgoCD apps and build kube-prometheus.
	createArgoCDRelatedFiles(clusterDir, repoDefaultBranchName, gitAuthMethod)

	// ArgoCD needs credentials to watch the kubeaid-config repo. These credentials will be stored in
	// a Sealed Secret.
	// Let's create that Sealed Secret file.
	createSealedSecretsRelatedFiles(clusterDir)

	// Add, commit and push the changes.
	commitHash := gitAddCommitAndPushChanges(repo, repoWorktree, branch, gitAuthMethod)

	// The user now needs to go ahead and create a PR from the new to the default branch. Then he
	// needs to merge that branch.
	// We can't create the PR for the user, since PRs are not part of the core git lib. They are
	// specific to the git platform the user is on.

	// Wait until the PR gets merged.
	waitUntilPRMerged(repo, repoDefaultBranchName, commitHash, gitAuthMethod, branch)

	// kubectl apply the root ArgoCD app.
	// NOTE: not using client-go lib on purpose, since we only need to kubectl apply 1 file.
	rootArgocdAppFilePath := fmt.Sprintf("%s/argocd-apps/templates/root.yaml", clusterDir)
	kubectlApplyCmd := parseCommand(fmt.Sprintf(
		"kubectl apply -f %s --kubeconfig %s", rootArgocdAppFilePath, config.ManagementClusterKubeconfig))
	log.Printf("Executing command : %v", kubectlApplyCmd)
	output, err = kubectlApplyCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("❌ Failed kubectl applying the root ArgoCD app : %v", err)
	}
	log.Print(string(output))

	log.Printf("💫 Finished running the kubeaid cluster bootstrap script")
}
