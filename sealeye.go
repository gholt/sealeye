// Package sealeye is a library for command line interfaces.
//
// See sealeye-example for complete examples of all the features, but a quick
// summary:
//
//  * Short options "-s" and long options "--long".
//  * Multiple option names per option.
//  * Boolean options can be flipped with "no" prefixing the long name, e.g. "--no-color".
//  * Environment variable defaults support.
//  * Multiple defaults support, for example "env:COUNT,123" which would use
//    the option's value if the user set it, or the COUNT environment variable
//    if that was set, or finally the plain value of 123 if all else failed.
//  * Subcommands using the exact same structures.
//  * Markdown support for help text, reformatting to fit the terminal and using color if possible.
//
// Things To Be Done Still:
//
//  * Clean up help text for options that override embedded struct options.
//  * Make an --all-help option to output all help for all subcommands.
//  * Support for other types: floats, times, durations, maybe lists.
//  * Handle --option=value format.
//  * Handle -abc to be the equivalent of -a -b -c but only for short options.
//  * Whatever other obvious stuff I'm forgetting because my brain is fried.
package sealeye

import (
	"fmt"
	"go/ast"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/gholt/blackfridaytext"
	"github.com/gholt/brimtext"
	"github.com/mattn/go-isatty"
)

// Run is the top-level sealeye handler. Usually, assumming your top-level
// command variable is named "root" this would be your main function:
//
//  func main() {
//      // This should be all you need for your main function.
//  	sealeye.Run(root)
//  }
func Run(cli interface{}) {
	runSubcommand(nil, os.Args[0], cli, os.Args[1:])
}

func runSubcommand(parent interface{}, name string, cli interface{}, args []string) {
	// Reflect down the value itself.
	reflectValue := reflect.ValueOf(cli)
	if reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	if parentField := reflectValue.FieldByName("Parent"); parentField.Kind() != reflect.Invalid {
		parentField.Set(reflect.ValueOf(parent))
	}

	// Parse out the overall help text -- the top part without the options.
	var helpText string
	helpTemplate, err := template.New("help").Parse(reflectValue.FieldByName("Help").String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse help text %q", reflectValue.FieldByName("Help").String())
		panic(err)
	}
	var helpBuilder strings.Builder
	if err := helpTemplate.Execute(&helpBuilder, map[string]interface{}{"Command": name}); err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse help text %q", reflectValue.FieldByName("Help").String())
		panic(err)
	}
	helpText = helpBuilder.String()

	// Parse out the options and their types. We just record the option types
	// as strings like, "bool", "int", etc. for simplicity as this really isn't
	// going to be a choke point.
	optionTypes := map[string]string{}
	optionValues := map[string]reflect.Value{}
	// Also, parse out the option help data, which is a table of each option
	// and its help text.
	var optionHelpData [][]string
	maxOptionLen := 0
	blankLinePending := false
	tty := 0
	var reflectFunc func(reflectType reflect.Type)
	reflectFunc = func(reflectType reflect.Type) {
		for i := 0; i < reflectType.NumField(); i++ {
			reflectField := reflectType.Field(i)
			if reflectField.Type.Kind() == reflect.Struct {
				reflectFunc(reflectField.Type)
			}
			if !ast.IsExported(reflectField.Name) {
				continue
			}
			optionTag := reflectField.Tag.Get("option")
			if optionTag == "" {
				continue
			}
			var optionType string
			switch reflectField.Type.Kind() {
			case reflect.Bool:
				optionType = "bool"
			case reflect.Int:
				optionType = "int"
			case reflect.String:
				optionType = "string"
			default:
				panic(fmt.Sprintln("cannot handle", reflectField.Name, reflectField.Type.Kind()))
			}
			var defaultsHelp []string
			for _, dflt := range strings.Split(reflectField.Tag.Get("default"), ",") {
				if dflt == "" {
					continue
				} else if strings.HasPrefix(dflt, "env:") {
					defaultsHelp = append(defaultsHelp, "$"+dflt[len("env:"):])
				} else if dflt == "terminal" {
					defaultsHelp = append(defaultsHelp, "if terminal")
				} else {
					defaultsHelp = append(defaultsHelp, dflt)
				}
			}
			var optionHelpNames []string
			for _, optionName := range strings.Split(optionTag, ",") {
				if optionName != "" {
					if len(optionName) == 1 {
						optionName = "-" + optionName
					} else {
						optionName = "--" + optionName
					}
					optionHelpName := optionName
					switch optionType {
					case "bool":
					case "int":
						optionHelpName += " n"
					case "string":
						optionHelpName += " s"
					default:
						panic(fmt.Sprintln("sealeye programmer error", optionType))
					}
					if len(optionHelpName) > maxOptionLen {
						maxOptionLen = len(optionHelpName)
					}
					optionHelpNames = append(optionHelpNames, optionHelpName)
					optionTypes[optionName] = optionType
					optionValues[optionName] = reflectValue.FieldByName(reflectField.Name)
				DEFAULTING:
					for _, dflt := range strings.Split(reflectField.Tag.Get("default"), ",") {
						if dflt == "" {
							continue
						} else if strings.HasPrefix(dflt, "env:") {
							if env, ok := os.LookupEnv(dflt[len("env:"):]); ok {
								switch optionType {
								case "bool":
									b, err := strconv.ParseBool(env)
									if err != nil {
										fmt.Fprintf(os.Stderr, "invalid boolean %q for option %q via $%s\n", env, optionName, dflt[len("env:"):])
										os.Exit(1)
									}
									optionValues[optionName].SetBool(b)
								case "int":
									i, err := strconv.ParseInt(env, 10, 64)
									if err != nil {
										fmt.Fprintf(os.Stderr, "invalid integer %q for option %q via $%s\n", env, optionName, dflt[len("env:"):])
										os.Exit(1)
									}
									optionValues[optionName].SetInt(i)
								case "string":
									optionValues[optionName].SetString(env)
								default:
									panic(fmt.Sprintln("sealeye programmer error", optionType))
								}
								break DEFAULTING
							}
						} else if dflt == "terminal" {
							if tty == 0 {
								if isatty.IsTerminal(os.Stdout.Fd()) {
									tty = 1
								} else {
									tty = -1
								}
							}
							optionValues[optionName].SetBool(tty == 1)
							break DEFAULTING
						} else {
							switch optionType {
							case "bool":
								b, err := strconv.ParseBool(dflt)
								if err != nil {
									panic(fmt.Sprintf("cannot handle default specification %q from %q: %s", dflt, reflectField.Tag.Get("default"), err))

								}
								optionValues[optionName].SetBool(b)
							case "int":
								i, err := strconv.ParseInt(dflt, 10, 64)
								if err != nil {
									panic(fmt.Sprintf("cannot handle default specification %q from %q: %s", dflt, reflectField.Tag.Get("default"), err))

								}
								optionValues[optionName].SetInt(i)
							case "string":
								optionValues[optionName].SetString(dflt)
							default:
								panic(fmt.Sprintln("sealeye programmer error", optionType))
							}
							break DEFAULTING
						}
					}
				}
			}
			if blankLinePending || len(optionHelpNames) > 1 {
				optionHelpData = append(optionHelpData, nil)
			}
			optionHelpText := reflectField.Tag.Get("help")
			if len(defaultsHelp) > 0 {
				optionHelpText += " Default: " + strings.Join(defaultsHelp, ", ")
			}
			optionHelpData = append(optionHelpData, []string{"", strings.Join(optionHelpNames, "\n"), optionHelpText})
			blankLinePending = len(optionHelpNames) > 1
		}
	}
	reflectFunc(reflectValue.Type())

	// Scan the command line for options and remaining args, possibly switching
	// context to a subcommand.
	var remainingArgs []string
	var subcommands map[string]interface{}
	if subcommandsField := reflectValue.FieldByName("Subcommands"); subcommandsField.Kind() != reflect.Invalid {
		subcommands, _ = subcommandsField.Interface().(map[string]interface{})
		if len(subcommands) == 0 {
			subcommands = nil
		}
	}
	noMore := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		addArg := func() {
			if subcommand, ok := subcommands[arg]; ok {
				runSubcommand(cli, name+" "+arg, subcommand, args[i+1:])
			}
			remainingArgs = append(remainingArgs, arg)
		}
		if noMore {
			addArg()
		}
		if len(arg) > 0 && arg[0] == '-' {
			switch optionTypes[arg] {
			case "bool":
				optionValues[arg].SetBool(true)
			case "int":
				if len(args) == i+1 {
					fmt.Fprintf(os.Stderr, "no value given for option %q\n", arg)
					os.Exit(1)
				}
				i++
				v, err := strconv.ParseInt(args[i], 10, 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "invalid int %q for option %q\n", args[i], arg)
					os.Exit(1)
				}
				optionValues[arg].SetInt(v)
			case "string":
				if len(args) == i+1 {
					fmt.Fprintf(os.Stderr, "no value given for option %q\n", arg)
					os.Exit(1)
				}
				i++
				optionValues[arg].SetString(args[i])
			default:
				if strings.HasPrefix(arg, "--no-") {
					arg2 := "--" + arg[len("--no-"):]
					if optionTypes[arg2] == "bool" {
						optionValues[arg2].SetBool(false)
						break
					}
				}
				fmt.Fprintf(os.Stderr, "unknown option %q\n", arg)
				os.Exit(1)
			}
		} else if arg == "--" {
			addArg()
			noMore = true
		} else {
			addArg()
		}
	}
	reflectValue.FieldByName("Args").Set(reflect.ValueOf(remainingArgs))

	// Output the full help text, if asked.
	helpFunc := func() {
		var color bool
		if colorOption := resolveOption(reflectValue, "Color"); colorOption.Kind() == reflect.Bool {
			color = colorOption.Bool()
		} else {
			color = isatty.IsTerminal(os.Stdout.Fd())
		}
		_, _ = os.Stdout.Write(blackfridaytext.MarkdownToTextNoMetadata([]byte(helpText), &blackfridaytext.Options{Color: color, TableAlignOptions: brimtext.NewUnicodeBoxedAlignOptions()}))
		alignOptions := brimtext.NewDefaultAlignOptions()
		alignOptions.RowSecondUD = "    "
		alignOptions.RowUD = "  "
		alignOptions.Widths = []int{4, 0, brimtext.GetTTYWidth() - maxOptionLen - 7}
		if len(optionHelpData) > 0 {
			fmt.Println()
			fmt.Println("Options:")
			fmt.Print(brimtext.Align(optionHelpData, alignOptions))
		}
		if subcommands != nil {
			fmt.Println()
			fmt.Println("Subcommands:")
			var subcommandNames []string
			maxSubcommandLen := 0
			for subcommandName := range subcommands {
				if len(subcommandName) > maxSubcommandLen {
					maxSubcommandLen = len(subcommandName)
				}
				subcommandNames = append(subcommandNames, subcommandName)
			}
			sort.Strings(subcommandNames)
			var subcommandHelpData [][]string
			for _, subcommandName := range subcommandNames {
				subcommandReflectValue := reflect.ValueOf(subcommands[subcommandName])
				if subcommandReflectValue.Kind() == reflect.Ptr {
					subcommandReflectValue = subcommandReflectValue.Elem()
				}
				subcommandHelpText := subcommandReflectValue.FieldByName("QuickHelp").String()
				subcommandHelpData = append(subcommandHelpData, []string{"", subcommandName, subcommandHelpText})
			}
			alignOptions.Widths = []int{4, maxSubcommandLen, brimtext.GetTTYWidth() - maxOptionLen - 7}
			fmt.Print(brimtext.Align(subcommandHelpData, alignOptions))
		}
	}
	if helpOption := reflectValue.FieldByName("HelpOption"); helpOption.Kind() == reflect.Bool && helpOption.Bool() {
		helpFunc()
		os.Exit(1)
	}

	// Actually Run!
	exitCode := int(reflectValue.FieldByName("Func").Call([]reflect.Value{reflect.ValueOf(cli)})[0].Int())
	if exitCode == 1 {
		helpFunc()
	}
	os.Exit(exitCode)
}

func resolveOption(reflectValue reflect.Value, name string) reflect.Value {
	if reflectValue.Kind() == reflect.Invalid {
		return reflectValue
	}
	if reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	rv := reflectValue.FieldByName(name)
	if rv.Kind() == reflect.Invalid {
		rv = resolveOption(reflectValue.FieldByName("Parent"), name)
	}
	return rv
}
