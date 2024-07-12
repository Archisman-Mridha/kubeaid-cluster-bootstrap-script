package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/charmbracelet/huh"
)

type InstallationCheck struct {
	name                     string
	macOSInstallationCommand *exec.Cmd
}

var installationChecks = []InstallationCheck{
	{
		name:                     "kubectl",
		macOSInstallationCommand: parseCommand("brew install kubectl"),
	},
	{
		name:                     "jsonnet",
		macOSInstallationCommand: parseCommand("brew install jsonnet"),
	},
	{
		name:                     "kubeseal",
		macOSInstallationCommand: parseCommand("brew install kubeseal"),
	},
	{
		name: "gojsontoyaml",
		macOSInstallationCommand: parseCommand(fmt.Sprintf(`
			cd %s && \
			mkdir temp && \
			cd temp && \
			wget https://github.com/brancz/gojsontoyaml/releases/download/v0.1.0/gojsontoyaml_0.1.0_darwin_arm64.tar.gz && \
			tar -xvzf gojsontoyaml_0.1.0_darwin_arm64.tar.gz && \
			chmod +x gojsontoyaml && \
			sudo mkdir -p /usr/local/bin && \
			sudo mv ./gojsontoyaml /usr/local/bin
		`, tempDirPath)),
	},
}

func ensurePrerequisitesInstalled() {
	log.Println("👀 Checking whether prerequisites are installed or not")

	for _, installationCheck := range installationChecks {
		if _, err := exec.Command("which", installationCheck.name).CombinedOutput(); err == nil {
			continue
		}

		log.Printf("❌ %s isn't installed in your system", installationCheck.name)

		var install bool
		huh.NewSelect[bool]().
			Title("Should I install it for you?").
			Options(
				huh.NewOption("yes", true),
				huh.NewOption("no", false),
			).
			Value(&install).
			Run()

		if !install {
			log.Fatalf("❌ Please install %s and rerun the script", installationCheck.name)
		}

		log.Printf("Installing %s, by executing command : %v ....", installationCheck.name, installationCheck.macOSInstallationCommand)
		output, err := installationCheck.macOSInstallationCommand.CombinedOutput()
		log.Print(string(output))
		if err != nil {
			log.Fatalf("❌ Failed installing %s : %v", installationCheck.name, err)
		}

		log.Printf("✅ Installed %s", installationCheck.name)
	}
}
