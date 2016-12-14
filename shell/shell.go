package shell

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type Shell struct {
	Done chan bool
	Path string
	Name string
}

type Command struct {
	F     func(args []string)
	Usage string
	Help  string
	Args  int
}

func (shell *Shell) Interact(commands map[string]Command) {
	usage := func() {
		fmt.Println("Commands:")
		for _, cmd := range commands {
			fmt.Printf(" - %-25s %s\n", cmd.Usage, cmd.Help)
		}
		fmt.Println(" - exit")
	}

	usage()

	in := bufio.NewReader(os.Stdin)
LOOP:
	for {
		fmt.Print(" $ ")
		input, err := in.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		// Trim trailing newline
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		args := strings.Split(input, " ")
		if args[0] == "torrent" {
		} else if args[0] == "close" {
			break LOOP
		} else {
			usage()
			continue
		}

		switch args[1] {

		case "help":
			usage()

		default:
			if len(args) <= 1 {
				fmt.Println("Not enough arguments..")
				continue
			}

			cmd, ok := commands[args[1]]
			if ok {
				numargs := len(args) - 1
				if numargs < cmd.Args {
					fmt.Printf("Not enough arguments given to %v. Needs at least %v, given %v\n", args[0], cmd.Args, numargs)
					fmt.Printf("Usage: %v\n", cmd.Usage)
				} else {
					cmd.F(args)
				}
			} else {
				fmt.Println("Unrecognized command. Type \"help\" to see available commands.")
				usage()
			}
		}
	}
	shell.Done <- true
}

func OptionBool(opt string) (bool, error) {
	switch strings.ToLower(opt) {
	case "on", "true":
		return true, nil
	case "off", "false":
		return false, nil
	default:
		return false, fmt.Errorf("Unknown state %s. Expect on or off. ", opt)
	}
}
