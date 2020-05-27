// Package sealeye is a library for command line interfaces.
//
// See sealeye-example for complete examples of all the features, but a quick
// summary:
//
//  * Short options "-s" and long options "--long", with fallback to "-long" to support Go-like flags.
//  * Multiple option names per option.
//  * Boolean options can be flipped with "no" prefixing the long name, e.g. "--no-color".
//  * Environment variable defaults support.
//  * Multiple defaults support, for example "env:COUNT,123" which would use
//    the option's value if the user set it, or the COUNT environment variable
//    if that was set, or finally the plain value of 123 if all else failed.
//  * Subcommands using the exact same structures.
//  * Options grouping, for DRY reuse, by simple struct embedding.
//  * Markdown support for help text, reformatting to fit the terminal and using color if possible.
//  * Support for an --all-help option to output all help for all subcommands.
//
// Things To Be Done Still:
//
//  * Support for other types: floats, times, durations, maybe lists.
//  * Handle --option=value format.
//  * Handle -abc to be the equivalent of -a -b -c but only for short options.
package sealeye

import (
	"fmt"
	"go/ast"
	"io"
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
//  	sealeye.Run(root)
//  }
func Run(cli interface{}) {
	os.Exit(runSubcommand(os.Stdout, os.Stderr, nil, os.Args[0], cli, os.Args[1:]))
}

func runSubcommand(stdout fdWriter, stderr io.Writer, parent interface{}, name string, cli interface{}, args []string) int {
	// Reflect down the value itself.
	reflectValue := reflect.ValueOf(cli)
	if reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	if parentField := reflectValue.FieldByName("Parent"); parentField.Kind() != reflect.Invalid {
		parentField.Set(reflect.ValueOf(parent))
	}

	// Establish the subcommands map.
	var subcommands map[string]interface{}
	if subcommandsField := reflectValue.FieldByName("Subcommands"); subcommandsField.Kind() != reflect.Invalid {
		subcommands, _ = subcommandsField.Interface().(map[string]interface{})
		if len(subcommands) == 0 {
			subcommands = nil
		}
	}

	// Parse out the overall help text -- the top part without the options.
	var helpText string
	helpTemplate, err := template.New("help").Parse(reflectValue.FieldByName("Help").String())
	if err != nil {
		fmt.Fprintf(stderr, "Could not parse help text %q", reflectValue.FieldByName("Help").String())
		panic(err)
	}
	var helpBuilder strings.Builder
	if err := helpTemplate.Execute(&helpBuilder, map[string]interface{}{"Command": name}); err != nil {
		fmt.Fprintf(stderr, "Could not parse help text %q", reflectValue.FieldByName("Help").String())
		panic(err)
	}
	helpText = helpBuilder.String()

	// Parse out the options and their types and requirements. We just record
	// the option types as strings like, "bool", "int", etc. for simplicity as
	// this really isn't going to be a performance choke point.
	optionTypes := map[string]string{}
	optionValues := map[string]reflect.Value{}
	optionReqs := map[string]map[string]bool{}
	reqCheck := func(optionName, value string) error {
		if optionReqs[optionName]["dir"] {
			if fi, err := os.Stat(value); err != nil || !fi.IsDir() {
				return fmt.Errorf("%s %q is not a directory", optionName, value)
			}
		}
		if optionReqs[optionName]["dirorfile"] {
			if _, err := os.Stat(value); err != nil {
				return fmt.Errorf("%s %q is not a directory or file", optionName, value)
			}
		}
		if optionReqs[optionName]["file"] {
			if fi, err := os.Stat(value); err != nil || fi.IsDir() {
				return fmt.Errorf("%s %q is not a file", optionName, value)
			}
		}
		return nil
	}
	// Also, parse out the option help data, which is a table of each option
	// and its help text.
	var optionHelpData [][]string
	var multilineOptionHelpData [][]string
	maxOptionLen := 0
	tty := 0
	topFields := map[string]bool{}
	for i := 0; i < reflectValue.Type().NumField(); i++ {
		topFields[reflectValue.Type().Field(i).Name] = true
	}
	var reflectFunc func(reflectType reflect.Type, embeddedStruct bool) int
	reflectFunc = func(reflectType reflect.Type, embeddedStruct bool) int {
		for i := 0; i < reflectType.NumField(); i++ {
			reflectField := reflectType.Field(i)
			if reflectField.Type.Kind() == reflect.Struct {
				if code := reflectFunc(reflectField.Type, true); code != 0 {
					return code
				}
			}
			// Skip fields in embedded structs that are overridden by the top
			// level struct.
			if embeddedStruct && topFields[reflectField.Name] {
				continue
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
			var reqsHelp []string
			for _, req := range strings.Split(reflectField.Tag.Get("required"), ",") {
				switch req {
				case "":
				case "dir":
					reqsHelp = append(reqsHelp, "must be a directory")
				case "dirorfile":
					reqsHelp = append(reqsHelp, "must be a directory or file")
				case "file":
					reqsHelp = append(reqsHelp, "must be a file")
				default:
					panic(fmt.Sprintf("unknown required value: %q", req))
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
					optionReqs[optionName] = map[string]bool{}
					for _, req := range strings.Split(reflectField.Tag.Get("required"), ",") {
						switch req {
						case "":
						case "dir", "dirorfile", "file":
							optionReqs[optionName][req] = true
						default:
							panic(fmt.Sprintf("unknown required value: %q", req))
						}
					}
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
										fmt.Fprintf(stderr, "invalid boolean %q for option %q via $%s\n", env, optionName, dflt[len("env:"):])
										return 1
									}
									optionValues[optionName].SetBool(b)
								case "int":
									i, err := strconv.ParseInt(env, 10, 64)
									if err != nil {
										fmt.Fprintf(stderr, "invalid integer %q for option %q via $%s\n", env, optionName, dflt[len("env:"):])
										return 1
									}
									optionValues[optionName].SetInt(i)
								case "string":
									if err := reqCheck(optionName, env); err != nil {
										fmt.Fprintln(stderr, err)
										return 1
									}
									optionValues[optionName].SetString(env)
								default:
									panic(fmt.Sprintln("sealeye programmer error", optionType))
								}
								break DEFAULTING
							}
						} else if dflt == "terminal" {
							if tty == 0 {
								if isatty.IsTerminal(stdout.Fd()) {
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
								if err := reqCheck(optionName, dflt); err != nil {
									fmt.Fprintln(stderr, err)
									return 1
								}
								optionValues[optionName].SetString(dflt)
							default:
								panic(fmt.Sprintln("sealeye programmer error", optionType))
							}
							break DEFAULTING
						}
					}
				}
			}
			optionHelpText := reflectField.Tag.Get("help")
			if len(reqsHelp) > 0 {
				optionHelpText += " Requirements: " + strings.Join(reqsHelp, ", ")
			}
			if len(defaultsHelp) > 0 {
				optionHelpText += " Default: " + strings.Join(defaultsHelp, ", ")
			}
			if len(optionHelpNames) == 1 {
				if optionHelpNames[0] != "--all-help" || subcommands != nil {
					optionHelpData = append(optionHelpData, []string{"", optionHelpNames[0], optionHelpText})
				}
			} else {
				if optionType == "bool" {
					if s := strings.Join(optionHelpNames, " "); len(s) < 15 {
						optionHelpData = append(optionHelpData, []string{"", s, optionHelpText})
					} else {
						multilineOptionHelpData = append(multilineOptionHelpData, []string{"", strings.Join(optionHelpNames, "\n"), optionHelpText})
					}
				} else {
					multilineOptionHelpData = append(multilineOptionHelpData, []string{"", strings.Join(optionHelpNames, "\n"), optionHelpText})
				}
			}
		}
		return 0
	}
	if code := reflectFunc(reflectValue.Type(), false); code != 0 {
		return code
	}

	// Scan the command line for options and remaining args, possibly switching
	// context to a subcommand.
	var remainingArgs []string
	// noMore will be set true if we encounter a "--" alone; conventionally
	// means "no more options follow".
	noMore := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		addArg := func() (bool, int) {
			if subcommand, ok := subcommands[arg]; ok {
				return true, runSubcommand(stdout, stderr, cli, name+" "+arg, subcommand, args[i+1:])
			}
			remainingArgs = append(remainingArgs, arg)
			return false, 0
		}
		if noMore {
			if ret, code := addArg(); ret {
				return code
			}
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			optionType, ok := optionTypes[arg]
			if !ok {
				// If we didn't find a match for the option, and it begins with
				// just a single dash, try it with a double-dash for backward
				// compatibility with Go's flag library which allows options
				// like -version to mean the more standard --version option.
				//
				// GLH: Note that this will have tension with treating -abc as
				// if it were -a -b -c as is standard with most CLIs. When code
				// is added to try to "explode" such aggregate short options,
				// it should take precedence over this code trying to treat it
				// as an --abc long option.
				if len(arg) > 1 && arg[1] != '-' {
					arg = "-" + arg
					optionType, ok = optionTypes[arg]
				}
				// If still didn't find a match for the option, and it happens
				// to be --all-help, just pretend it was --help.
				if !ok && arg == "--all-help" {
					arg = "--help"
					optionType = optionTypes[arg]
				}
			}
			switch optionType {
			case "bool":
				optionValues[arg].SetBool(true)
			case "int":
				if len(args) == i+1 {
					fmt.Fprintf(stderr, "no value given for option %q\n", arg)
					return 1
				}
				i++
				v, err := strconv.ParseInt(args[i], 10, 64)
				if err != nil {
					fmt.Fprintf(stderr, "invalid int %q for option %q\n", args[i], arg)
					return 1
				}
				optionValues[arg].SetInt(v)
			case "string":
				if len(args) == i+1 {
					fmt.Fprintf(stderr, "no value given for option %q\n", arg)
					return 1
				}
				i++
				if err := reqCheck(arg, args[i]); err != nil {
					fmt.Fprintln(stderr, err)
					return 1
				}
				optionValues[arg].SetString(args[i])
			default:
				if strings.HasPrefix(arg, "--no-") {
					arg2 := "--" + arg[len("--no-"):]
					if optionTypes[arg2] == "bool" {
						optionValues[arg2].SetBool(false)
						break
					}
				}
				fmt.Fprintf(stderr, "unknown option %q\n", arg)
				return 1
			}
		} else if arg == "--" {
			noMore = true
		} else {
			if ret, code := addArg(); ret {
				return code
			}
		}
	}
	reflectValue.FieldByName("Args").Set(reflect.ValueOf(remainingArgs))

	// Output the full help text, if asked.
	helpFunc := func() {
		var color bool
		if colorOption := resolveOption(reflectValue, "Color"); colorOption.Kind() == reflect.Bool {
			color = colorOption.Bool()
		} else {
			color = isatty.IsTerminal(stdout.Fd())
		}
		_, _ = stdout.Write(blackfridaytext.MarkdownToTextNoMetadata([]byte(helpText), &blackfridaytext.Options{Color: color, TableAlignOptions: brimtext.NewUnicodeBoxedAlignOptions()}))
		alignOptions := brimtext.NewDefaultAlignOptions()
		alignOptions.RowSecondUD = "    "
		alignOptions.RowUD = "  "
		alignOptions.Widths = []int{4, 0, brimtext.GetTTYWidth() - maxOptionLen - 8}
		if len(optionHelpData) > 0 || len(multilineOptionHelpData) > 0 {
			// Sort help and all-help to the top, dictionary order after that.
			sort.Slice(optionHelpData, func(i, j int) bool {
				si := strings.ToLower(strings.TrimLeft(optionHelpData[i][1], "-"))
				if si[0] == '?' {
					return true
				}
				sj := strings.ToLower(strings.TrimLeft(optionHelpData[j][1], "-"))
				if si == "all-help" {
					return sj[0] != '?'
				}
				if sj == "all-help" {
					return false
				}
				return si < sj
			})
			// Sort all multiline options in dictionary order.
			sort.Slice(multilineOptionHelpData, func(i, j int) bool {
				return multilineOptionHelpData[i][1] < multilineOptionHelpData[j][1]
			})
			for _, helpData := range multilineOptionHelpData {
				optionHelpData = append(optionHelpData, nil, helpData)
			}
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
	if allHelpOption := reflectValue.FieldByName("AllHelpOption"); allHelpOption.Kind() == reflect.Bool && allHelpOption.Bool() {
		helpFunc()
		var subcommandNames []string
		for subcommandName := range subcommands {
			subcommandNames = append(subcommandNames, subcommandName)
		}
		sort.Strings(subcommandNames)
		for _, subcommandName := range subcommandNames {
			s := "---[ " + name + " " + subcommandName + " ]"
			fmt.Fprintln(stdout)
			fmt.Fprint(stdout, s)
			fmt.Fprintln(stdout, strings.Repeat("-", brimtext.GetTTYWidth()-len(s)-1))
			fmt.Fprintln(stdout)
			runSubcommand(stdout, stderr, cli, name+" "+subcommandName, subcommands[subcommandName], []string{"--all-help"})
		}
		return 1
	}
	if helpOption := reflectValue.FieldByName("HelpOption"); helpOption.Kind() == reflect.Bool && helpOption.Bool() {
		helpFunc()
		return 1
	}

	// Actually Run!
	exitCode := int(reflectValue.FieldByName("Func").Call([]reflect.Value{reflect.ValueOf(cli)})[0].Int())
	if exitCode == 1 {
		helpFunc()
	}
	return exitCode
}

func resolveOption(reflectValue reflect.Value, name string) reflect.Value {
	if reflectValue.Kind() == reflect.Invalid {
		return reflectValue
	}
	for {
		if reflectValue.Kind() == reflect.Interface || reflectValue.Kind() == reflect.Ptr {
			reflectValue = reflectValue.Elem()
		} else {
			break
		}
	}
	rv := reflectValue.FieldByName(name)
	if rv.Kind() == reflect.Invalid {
		rv = resolveOption(reflectValue.FieldByName("Parent"), name)
	}
	return rv
}

type fdWriter interface {
	io.Writer
	Fd() uintptr
}
