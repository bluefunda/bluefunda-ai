package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/memory"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var (
	memoryDeleteForce bool
	memoryListAll     bool
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage persistent agent memory (.bai/memory, ~/.bai/memory)",
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memory keys with a one-line preview",
	RunE:  runMemoryList,
}

var memoryShowCmd = &cobra.Command{
	Use:   "show <key>",
	Short: "Print the full content of a memory entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemoryShow,
}

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a project-level memory entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemoryDelete,
}

func init() {
	memoryDeleteCmd.Flags().BoolVarP(&memoryDeleteForce, "force", "f", false, "Skip the confirmation prompt")
	memoryListCmd.Flags().BoolVar(&memoryListAll, "all", false, "Include superseded entries")
	memoryCmd.AddCommand(memoryListCmd, memoryShowCmd, memoryDeleteCmd)
}

func runMemoryList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return listMemory(memory.New(cwd), printer(loadConfig()), memoryListAll)
}

func listMemory(mgr *memory.Manager, p *ui.Printer, all bool) error {
	list := mgr.List
	if all {
		list = mgr.ListAll
	}
	entries, err := list()
	if err != nil {
		return fmt.Errorf("list memory: %w", err)
	}
	if len(entries) == 0 {
		p.Info("no memory entries")
		return nil
	}

	headers := []string{"KEY", "SCOPE", "STATUS", "PREVIEW"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{e.Key, e.Scope, e.Status, e.Preview()})
	}
	p.Table(headers, rows)
	return nil
}

func runMemoryShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return showMemory(memory.New(cwd), args[0], cmd.OutOrStdout())
}

func showMemory(mgr *memory.Manager, key string, out io.Writer) error {
	e, err := mgr.Read(key)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, e.Content)
	return nil
}

func runMemoryDelete(cmd *cobra.Command, args []string) error {
	key := args[0]
	if !memoryDeleteForce && !confirmMemoryDelete(cmd.InOrStdin(), cmd.OutOrStdout(), key) {
		fmt.Fprintln(cmd.OutOrStdout(), "aborted")
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return deleteMemory(memory.New(cwd), key, printer(loadConfig()))
}

func deleteMemory(mgr *memory.Manager, key string, p *ui.Printer) error {
	if err := mgr.Delete(key); err != nil {
		return err
	}
	p.Success(fmt.Sprintf("deleted memory %q", key))
	return nil
}

// confirmMemoryDelete prompts on out and reads a single line from in.
// Returns true only for an explicit y/yes response.
func confirmMemoryDelete(in io.Reader, out io.Writer, key string) bool {
	fmt.Fprintf(out, "Delete memory %q? [y/N] ", key)
	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
