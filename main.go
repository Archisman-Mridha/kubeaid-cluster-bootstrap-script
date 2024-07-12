package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

type (
	Config struct {
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
	}

	ArgocdAppTemplateValues struct {
		ClusterName,
		KubeAidRepo,
		KubeAidConfigRepo,
		Branch string
	}

	JsonnetFileTemplateValues struct {
		ConnectObmondo        bool
		KubePrometheusVersion string
		GrafanaURL            string
	}

	SealedSecretArgocdRepoCredentialsTemplateValues struct {
		Name     string
		Password string
		Type     string
		URL      string
		Username string
	}
)

var (
	currentTime = time.Now().Unix()

	tempDirPath = getTempDirPath()
	repoDir     = tempDirPath + "/kubeaid-config"

	config Config

	defaultArgocdApps = []string{
		"root",
		"argo-cd",
		"cilium",
		"cluster-api",
		"kube-prometheus",
		"sealed-secrets",
		"traefik",
	}
)

func main() {
	// Delete the temp dir after the script finishes running.
	defer os.RemoveAll(tempDirPath)

	log.SetLevel(log.DebugLevel)

	log.Infof("💫 Running the kubeaid cluster bootstrap script")

	// Declare and parse CLI flags.
	configFile := flag.String("config-file", "", "Path to the YAML config file")
	flag.Parse()

	parseConfigFile(configFile)

	gitAuthMethod := getGitAuthMethod()

	// Clone kubeaid-config repo.
	repo := gitCloneRepo(config.KubeaidConfigRepoURL, repoDir, gitAuthMethod)
	repoDefaultBranchName := getDefaultBranchName(repo)

	// Create and checkout to a new branch.
	repoWorktree, err := repo.Worktree()
	if err != nil {
		log.Fatal("Failed getting kubeaid-config repo worktree")
	}
	branch := fmt.Sprintf("kubeaid-%s-%d", config.ClusterName, currentTime)
	createAndCheckoutToBranch(repo, branch, repoWorktree)

	// In the k8s dir, we will create a folder for the cluster. Files related to the cluster, will
	// be generated in this folder.
	clusterDir := fmt.Sprintf("%s/k8s/%s", repoDir, config.ClusterName)
	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		log.Fatalf("Cluster dir %s already exists", clusterDir)
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
	kubectlApplyCmd := exec.Command(
		"kubectl", "apply",
		"-f", rootArgocdAppFilePath,
		"--kubeconfig", config.ManagementClusterKubeconfig,
	)
	log.Debug("Executing command : %v", kubectlApplyCmd)
	output, err := kubectlApplyCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed kubectl applying the root ArgoCD app : %v", err)
	}
	log.Debugf("Output of kubectl applying the root argocd app : %v", output)

	log.Infof("💫 Finished running the kubeaid cluster bootstrap script")
}
