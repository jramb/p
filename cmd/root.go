package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

var EffectiveTimeNow = time.Now() //.round(time.Minute)

var cfgFile string
var Clockfile string
var ModifyEffectiveTime time.Duration
var RoundTime int

var Debug bool

func D(args ...interface{}) {
	if Debug {
		//log.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
		fmt.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
	}
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "p",
	Short: "Punch or short 'p': A flexible time tracker and time reporting tool",
	Long: `Punch or short 'p': A flexible time tracker and time reporting tool.
made by (and mainly for) Jörg Ramb.

Use this tool to keep track of time spent on projects
and assignments. Apart from registering the time periods in a database you
can use this to perform simple housekeeping and reporting on the data.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.punch.yaml)")
	RootCmd.PersistentFlags().StringVarP(&Clockfile, "clockfile", "c", "", "Path to the clockfile = time entry database")
	RootCmd.PersistentFlags().BoolVarP(&Debug, "verbose", "v", false, "Enables verbose output")
	RootCmd.PersistentFlags().DurationVarP(&ModifyEffectiveTime, "mod", "m", time.Duration(0), "Modify effective time (backwards)")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	viper.BindPFlag("clockfile", RootCmd.PersistentFlags().Lookup("clockfile"))
	viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose"))
	//fmt.Println("2clockfile=", viper.GetString("clockfile"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".punch") // name of config file (without extension)
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME") // adding home directory as first search path
	//viper.AutomaticEnv()         // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		D("Using config file: " + viper.ConfigFileUsed())
	}
	//RootCmd.DebugFlags()
}
