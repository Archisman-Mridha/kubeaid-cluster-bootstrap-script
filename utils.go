package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"gopkg.in/yaml.v3"
)

func parseConfigFile(configFile *string) {
	configFileContents, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("❌ Failed reading config file : %v", err)
	}
	err = yaml.Unmarshal(configFileContents, &config)
	if err != nil {
		log.Fatalf("❌ Failed unmarshalling config file : %v", err)
	}
	log.Println("✅ Parsed config from the config file")
}

func getTempDirPath() string {
	name := fmt.Sprintf("kubeaid-bootstrap-script-%d", currentTime)
	path, err := os.MkdirTemp("/tmp", name)
	if err != nil {
		log.Fatalf("❌ Failed creating temp dir : %v", err)
	}
	log.Printf("📁 Created temp dir %s", path)
	return path
}

func getGitAuthMethod() (authMethod transport.AuthMethod) {
	if len(config.Git.SSHPrivateKey) > 0 {
		publicKeys, err := ssh.NewPublicKeysFromFile("git", config.Git.SSHPrivateKey, config.Git.Password)
		if err != nil {
			log.Fatalf("❌ Failed generating SSH public key from SSH private key and password for git : %v", err)
		}
		authMethod = publicKeys
		log.Println("🔑 Using SSH private key and password for git authentication")
		return
	}

	if len(config.Git.Password) > 0 {
		authMethod = &http.BasicAuth{
			Username: config.Git.Username,
			Password: config.Git.Password,
		}
		log.Println("🔑 Using password for git authentication")
		return
	}

	sshAuth, err := ssh.NewSSHAgentAuth("git")
	if err != nil {
		log.Fatalf("❌ ssh agent failed : %v", err)
	}
	authMethod = sshAuth
	log.Println("🔑 Using SSH agent for git authentication")
	return
}

func gitCloneRepo(url, dir string, authMethod transport.AuthMethod) *git.Repository {
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		Auth: authMethod,
		URL:  url,
	})
	if err != nil {
		log.Fatalf("❌ Failed git cloning repo %s in %s : %v", url, dir, err)
	}
	log.Printf("✅ Cloned repo %s in %s", url, dir)
	return repo
}

func getDefaultBranchName(repo *git.Repository) string {
	headRef, err := repo.Head()
	if err != nil {
		log.Fatal("❌ Failed getting HEAD ref of kubeaid-config repo")
	}
	return headRef.Name().Short()
}

func createAndCheckoutToBranch(repo *git.Repository, branch string, workTree *git.Worktree) {
	// Check if the branch already exists.
	branchRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+branch), true)
	if err == nil && branchRef != nil {
		log.Fatalf("❌ Branch '%s' already exists in the kubeaid-config repo", branch)
	}

	if err = workTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branch),
		Create: true,
	}); err != nil {
		log.Fatalf("❌ Failed creating branch '%s', in kubeaid-config repo : %v", branch, err)
	}
	log.Printf("✅ Created branch '%s' in the kubeaid-config repo", branch)
}

func gitAddCommitAndPushChanges(repo *git.Repository, workTree *git.Worktree, branch string, auth transport.AuthMethod) plumbing.Hash {
	if err := workTree.AddGlob(fmt.Sprintf("k8s/%s/*", config.ClusterName)); err != nil {
		log.Fatalf("❌ Failed adding changes to git : %v", err)
	}

	status, err := workTree.Status()
	if err != nil {
		log.Fatalf("❌ Failed determining git status : %v", err)
	}
	log.Printf("git status : %v\n", status)

	commitMessage := fmt.Sprintf("KubeAid bootstrap setup for argo-cd applications on %s\n", config.ClusterName)
	commit, err := workTree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "KubeAid Installer",
			Email: "info@obmondo.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Fatalf("❌ Failed creating git commit : %v", err)
	}
	commitObject, err := repo.CommitObject(commit)
	if err != nil {
		log.Fatalf("❌ Failed getting commit object : %v", err)
	}
	log.Printf("git commit object : %v", commitObject)

	if err = repo.Push(&git.PushOptions{
		Progress:   os.Stdout,
		RemoteName: "origin",
		RefSpecs: []gitConfig.RefSpec{
			gitConfig.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch),
		},
		Auth: auth,
	}); err != nil {
		log.Fatalf("❌ git push failed : %v", err)
	}

	log.Printf("✅ Added, committed and pushed changes | Commit hash = %s", commitObject.Hash)
	return commitObject.Hash
}

func waitUntilPRMerged(repo *git.Repository, defaultBranchName string, commitHash plumbing.Hash, auth transport.AuthMethod, branchToBeMerged string) {
	for {
		log.Printf("👀 Waiting for %s branch to be merged into the default branch %s. Sleeping for 10 seconds...\n", branchToBeMerged, defaultBranchName)
		time.Sleep(10 * time.Second)

		if err := repo.Fetch(&git.FetchOptions{
			Auth:     auth,
			RefSpecs: []gitConfig.RefSpec{"refs/*:refs/*"},
		}); err != nil && err != git.NoErrAlreadyUpToDate {
			log.Fatalf("❌ Failed determining whether branch is merged or not : %v", err)
		}

		defaultBranchRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+defaultBranchName), true)
		if err != nil {
			log.Fatalf("❌ Failed to get default branch ref of kubeaid-config repo : %v", err)
		}

		if commitPresent := isCommitPresentInBranch(repo, commitHash, defaultBranchRef.Hash()); commitPresent {
			log.Printf("✅ Detected branch merge")
			return
		}
	}
}

func copyFile(sourceFile, destinationFile string) error {
	src, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}

func isCommitPresentInBranch(repo *git.Repository, commitHash, branchHash plumbing.Hash) bool {
	// Iterate through the commit history of the branch
	commits, err := repo.Log(&git.LogOptions{From: branchHash})
	if err != nil {
		log.Fatalf("Failed git logging : %v", err)
	}

	for {
		c, err := commits.Next()
		if err != nil {
			break
		}

		if c.Hash == commitHash {
			return true
		}
	}

	return false
}

func encodeStringToBase64(input string) string {
	data := []byte(input)
	encodedString := base64.StdEncoding.EncodeToString(data)
	return encodedString
}

func parseCommand(command string) *exec.Cmd {
	return exec.Command("bash", "-c", command)
}
