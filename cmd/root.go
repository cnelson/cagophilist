package cmd // import "electric-it.io/cago"

import (
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/cavaliercoder/grab"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	localConfigFile  = ".cago/cago.yaml"
	cachedConfigFile = ".cago/cached.cago.yaml"

	remoteConfigFileEnvVariable = "CAGO_CONFIG_URL"

	configFileFlagLong  = "config-file"
	configFileFlagShort = "c"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "cago",
	Short: "Cagophilist (cago for short) helps manage AWS profiles that are linked to a SAML identity provider",
	Long:  ``,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	executeError := RootCmd.Execute()
	if executeError != nil {
		log.Fatalf("Unable to execute the command: %s", executeError)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug output")
	err := viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	if err != nil {
		log.Fatalf("Error binding pflag %s: %s", "debug", err)
	}

	RootCmd.PersistentFlags().BoolP("insecure", "k", false, "Ignore SSL certificates")
	err = viper.BindPFlag("insecure", RootCmd.PersistentFlags().Lookup("insecure"))
	if err != nil {
		log.Fatalf("Error binding pflag %s: %s", "insecure", err)
	}

	RootCmd.PersistentFlags().BoolP("ignore-proxy-config", "i", false, "Ignore proxies in the configuration file")
	err = viper.BindPFlag("ignore-proxy-config", RootCmd.PersistentFlags().Lookup("ignore-proxy-config"))
	if err != nil {
		log.Fatalf("Error binding pflag %s: %s", "ignore-proxy-config", err)
	}

	RootCmd.PersistentFlags().BoolP("prompt-for-credentials", "p", false, "Always prompt for credentials, never use the cache")
	err = viper.BindPFlag("prompt-for-credentials", RootCmd.PersistentFlags().Lookup("prompt-for-credentials"))
	if err != nil {
		log.Fatalf("Error binding pflag %s: %s", "prompt-for-credentials", err)
	}

	RootCmd.PersistentFlags().StringP(configFileFlagLong, configFileFlagShort, "", "Local path to the configuration file")
	err = viper.BindPFlag(configFileFlagLong, RootCmd.PersistentFlags().Lookup(configFileFlagLong))
	if err != nil {
		log.Fatalf("Error binding pflag %s: %s", configFileFlagLong, err)
	}

	log.SetHandler(cli.New(os.Stderr))
}

// initConfig loads the config file in the following precidence: CAGO_CONFIG_URL env variable, user home directory
func initConfig() {
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
		log.Debugf("Debug logging enabled!")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.Debugf("Cago Version: %s", Version)

	// Enable overriding of configuration values using environment variables
	viper.SetEnvPrefix("CAGO")
	viper.AutomaticEnv()

	// Check for the configuration file in the following order:
	//  1. Local configuration file specified using a command line argument
	//  2. Remote configuration file specified using the CAGO_CONFIG_URL environment variable
	//  3. Local configuration file cached from a previous download
	//  4. Local configuration file manually created by user

	// Step 1: Check to see if the configuration file path is set via command line argument
	configurationFilePath := viper.GetString(configFileFlagLong)
	if configurationFilePath != "" {
		log.Debugf("Configuration file command line argument '%s' is set to: %s", configFileFlagLong, configurationFilePath)
	} else {
		log.Debugf("Configuration file command line argument '%s' is not set", configFileFlagLong)
	}

	// Step 2 and 3: Download a remote file or used a previously cached version
	if configurationFilePath == "" {
		remoteConfigurationFileURL, ok := os.LookupEnv("CAGO_CONFIG_URL")
		if ok {
			log.Debugf("Environment variable '%s' is set to: %s", remoteConfigFileEnvVariable, remoteConfigurationFileURL)
			configurationFilePath = getRemoteConfigurationFile(remoteConfigurationFileURL)
		} else {
			log.Debugf("Environment variable '%s' is not set", remoteConfigFileEnvVariable)
		}
	}

	// Step 4: Use a manually created local file
	if configurationFilePath == "" {
		// If the  didn't load, try finding the remote configuration file
		configurationFilePath = getLocalConfigurationFile()
	}

	if configurationFilePath == "" {
		log.Errorf("Cago could not find a configuration file to use! Here's what Cago checks:")
		log.Errorf("  1. Configuration file path specified using command line argument: %s", configFileFlagLong)
		log.Errorf("  2. Remote configuration file URL specified using environment variable: %s", remoteConfigFileEnvVariable)
		log.Errorf("  3. Previously cached remote configuration file in: %s", cachedConfigFile)
		log.Errorf("  4. Manually created configuration file here: %s", localConfigFile)

		os.Exit(1)
	}

	// Read the configuration file
	viper.SetConfigFile(configurationFilePath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Could not process configuration file (%s), bailing out: %s", configurationFilePath, err)
		os.Exit(1)
	}
}

// getLocalConfigurationFile returns the local configuration file if it exists, sets ok to false otherwise
func getLocalConfigurationFile() (configurationFilePath string) {
	// Open the user's home directory
	homeDirPath, homedirError := homedir.Dir()
	if homedirError != nil {
		log.Fatalf("Unable to open user's home directory, bailing out: %s", homedirError)
		os.Exit(1)
	}

	// This is where the file should exist
	localConfigFileLocation := filepath.Join(homeDirPath, localConfigFile)

	// Check to see if the cached file exists
	if _, statError := os.Stat(localConfigFileLocation); statError != nil {
		log.Debugf("Unable to find local configuration file: %s", localConfigFileLocation)

		return ""
	}

	log.Debugf("Found local configuration file: %s", localConfigFileLocation)

	return localConfigFileLocation
}

func getRemoteConfigurationFile(url string) (configurationFilePath string) {
	// Open the user's home directory
	homedirpath, homedirError := homedir.Dir()
	if homedirError != nil {
		log.Fatalf("I can't access the user's home directory, which is where I want to write the configuration file: %s", homedirError)
		os.Exit(1)
	}

	// Download the latest configuration file
	downloadedConfigFileLocation := filepath.Join(homedirpath, cachedConfigFile)
	log.Debugf("Attempting to download configuration file to %s", downloadedConfigFileLocation)

	_, getError := grab.Get(downloadedConfigFileLocation, url)
	if getError != nil {
		log.Errorf("Failed to download configuration file: %s", getError)
	}

	// Check to see if the file was downloaded or at least a previous download exists
	if _, statError := os.Stat(downloadedConfigFileLocation); statError != nil {
		log.Debugf("Unable to find downloaded configuration file: %s", downloadedConfigFileLocation)

		return ""
	}

	log.Debugf("Configuration file downloaded to %s", downloadedConfigFileLocation)

	return downloadedConfigFileLocation
}
