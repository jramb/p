package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jramb/chalk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var EffectiveTimeNow = time.Now() //.round(time.Minute)

var cfgFile string
var clockfile string
var ModifyEffectiveTime time.Duration
var OrgMode bool

var Debug bool

func D(args ...interface{}) {
	//if viper.GetBool("debug") {
	if Debug {
		//log.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
		fmt.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
	}
}

func GetEffectiveTime() time.Time {
	effectiveTimeNow := time.Now()

	//if ModifyEffectiveTime != nil {
	effectiveTimeNow = effectiveTimeNow.Add(-ModifyEffectiveTime).Round(time.Minute)
	//}
	// Rounding is not performed during entry
	//if roundTime != nil {
	//effectiveTimeNow = effectiveTimeNow.Round(RoundTime)
	//}
	D("Effective time: " + effectiveTimeNow.Format("2006-01-02 15:04"))
	return effectiveTimeNow
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "p",
	Short: "Punch or short 'p': A flexible time tracker and time reporting tool",
	Long: `Punch or short 'p': A flexible time tracker and time reporting tool.
made by (and mainly for) Jörg Ramb.

Use this tool to keep track of time spent on projects, assignments, work, etc.
Apart from registering the time periods in a database you
can use this to perform simple todo, logging and reporting on the data.`,
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
	RootCmd.PersistentFlags().StringVarP(&clockfile, "clockfile", "c", "", "Path to the clockfile = time entry database")
	RootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "D", false, "Enables debug output")
	RootCmd.PersistentFlags().DurationVarP(&ModifyEffectiveTime, "mod", "m", time.Duration(0), "modify effective time (backwards), eg 7m subtracts 7 minutes")
	RootCmd.PersistentFlags().DurationVarP(&RoundTime, "rounding", "", time.Minute, "round times according to this duration, e.g. 1m, 15m, 1h")
	RootCmd.PersistentFlags().IntVarP(&RoundingBias, "bias", "", 0, "rounding bias (default 0=absolute fair,1,2, max 3=alwas round up")
	RootCmd.PersistentFlags().BoolVarP(&OrgMode, "orgmode", "o", false, "use OrgMode format where applicable")
	RootCmd.PersistentFlags().BoolVarP(&ShowRounding, "display-rounding", "r", false, "display rounding difference in output")
	RootCmd.PersistentFlags().StringVarP(&DurationStyle, "style", "", "hour", "show duration style: time (2:30)/ hour (2.5 h) / short (2.5, default)")
	RootCmd.PersistentFlags().BoolVarP(&SubHeaders, "subheaders", "s", false, "display subheaders")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	viper.BindPFlag("clockfile", RootCmd.PersistentFlags().Lookup("clockfile"))
	viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	//fmt.Println("2clockfile=", viper.GetString("clockfile"))
	viper.BindPFlag("show.rounding", RootCmd.PersistentFlags().Lookup("rounding"))
	viper.BindPFlag("show.style", RootCmd.PersistentFlags().Lookup("style"))
	viper.BindPFlag("show.bias", RootCmd.PersistentFlags().Lookup("bias"))
	viper.BindPFlag("show.orgmode", RootCmd.PersistentFlags().Lookup("orgmode"))
	viper.BindPFlag("show.display-rounding", RootCmd.PersistentFlags().Lookup("display-rounding"))
	viper.BindPFlag("show.subheaders", RootCmd.PersistentFlags().Lookup("subheaders"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName("punch")                                    // name of config file (without extension)
	viper.AddConfigPath(".")                                        //current
	viper.AddConfigPath("$HOME/.config")                            //config directory
	viper.AddConfigPath("$HOME")                                    // adding home directory as first search path
	if userprofile := os.Getenv("USERPROFILE"); userprofile != "" { //Windows
		viper.AddConfigPath(userprofile)
	}
	//viper.AutomaticEnv()         // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		D("Using config file: " + viper.ConfigFileUsed())
	}
	//RootCmd.DebugFlags()
}
