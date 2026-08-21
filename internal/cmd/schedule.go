package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/schedule"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var (
	scheduleCron  string
	scheduleName  string
	scheduleDir   string
	scheduleModel string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage recurring agent runs (Phase 1, client-side — see `bai daemon`)",
}

var scheduleCreateCmd = &cobra.Command{
	Use:   "create <prompt>",
	Short: "Create a scheduled agent run",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleCreate,
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled agent runs",
	RunE:  runScheduleList,
}

var scheduleShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details of a scheduled run",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleShow,
}

var scheduleDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a scheduled run",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleDelete,
}

func init() {
	scheduleCreateCmd.Flags().StringVar(&scheduleCron, "cron", "", `Cron expression, standard 5-field syntax (e.g. "0 9 * * 1-5") — required`)
	scheduleCreateCmd.Flags().StringVar(&scheduleName, "name", "", "Optional display name")
	scheduleCreateCmd.Flags().StringVar(&scheduleDir, "dir", "", "Working directory for the run (default: current directory)")
	scheduleCreateCmd.Flags().StringVar(&scheduleModel, "model", "", "Model alias for the run (default: config default)")
	_ = scheduleCreateCmd.MarkFlagRequired("cron")

	scheduleCmd.AddCommand(scheduleCreateCmd, scheduleListCmd, scheduleShowCmd, scheduleDeleteCmd)
}

func scheduleStore() (*schedule.Store, error) {
	path, err := schedule.DefaultPath()
	if err != nil {
		return nil, err
	}
	return schedule.New(path), nil
}

func runScheduleCreate(cmd *cobra.Command, args []string) error {
	store, err := scheduleStore()
	if err != nil {
		return err
	}
	dir := scheduleDir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}
	return createSchedule(store, printer(loadConfig()), schedule.Entry{
		Name:    scheduleName,
		Cron:    scheduleCron,
		Prompt:  args[0],
		Dir:     dir,
		Model:   scheduleModel,
		Enabled: true,
	})
}

// createSchedule is runScheduleCreate's testable core.
func createSchedule(store *schedule.Store, p *ui.Printer, e schedule.Entry) error {
	created, err := store.Create(e)
	if err != nil {
		return err
	}
	p.Success(fmt.Sprintf("Created schedule %s — next run: %s", created.ID, formatScheduleTime(created.NextRun)))
	p.Info("Run `bai daemon start` for scheduled runs to actually fire.")
	return nil
}

func runScheduleList(cmd *cobra.Command, args []string) error {
	store, err := scheduleStore()
	if err != nil {
		return err
	}
	return listSchedules(store, printer(loadConfig()))
}

// listSchedules is runScheduleList's testable core.
func listSchedules(store *schedule.Store, p *ui.Printer) error {
	entries, err := store.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		p.Info("no scheduled runs — create one with `bai schedule create --cron \"...\" \"prompt\"`")
		return nil
	}

	headers := []string{"ID", "NAME", "CRON", "ENABLED", "NEXT RUN", "LAST RUN", "STATUS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.ID,
			e.Name,
			e.Cron,
			fmt.Sprintf("%v", e.Enabled),
			formatScheduleTime(e.NextRun),
			formatScheduleTime(e.LastRun),
			e.LastStatus,
		})
	}
	p.Table(headers, rows)
	return nil
}

func runScheduleShow(cmd *cobra.Command, args []string) error {
	store, err := scheduleStore()
	if err != nil {
		return err
	}
	return showSchedule(store, cmd.OutOrStdout(), args[0])
}

// showSchedule is runScheduleShow's testable core.
func showSchedule(store *schedule.Store, out io.Writer, id string) error {
	e, ok, err := store.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no schedule found with id %q", id)
	}

	fmt.Fprintf(out, "ID:        %s\n", e.ID)
	if e.Name != "" {
		fmt.Fprintf(out, "Name:      %s\n", e.Name)
	}
	fmt.Fprintf(out, "Cron:      %s\n", e.Cron)
	fmt.Fprintf(out, "Enabled:   %v\n", e.Enabled)
	fmt.Fprintf(out, "Dir:       %s\n", e.Dir)
	if e.Model != "" {
		fmt.Fprintf(out, "Model:     %s\n", e.Model)
	}
	fmt.Fprintf(out, "Next run:  %s\n", formatScheduleTime(e.NextRun))
	fmt.Fprintf(out, "Last run:  %s\n", formatScheduleTime(e.LastRun))
	if e.LastStatus != "" {
		fmt.Fprintf(out, "Status:    %s\n", e.LastStatus)
	}
	if e.LastError != "" {
		fmt.Fprintf(out, "Error:     %s\n", e.LastError)
	}
	fmt.Fprintf(out, "\nPrompt:\n%s\n", e.Prompt)
	return nil
}

func runScheduleDelete(cmd *cobra.Command, args []string) error {
	store, err := scheduleStore()
	if err != nil {
		return err
	}
	return deleteSchedule(store, printer(loadConfig()), args[0])
}

// deleteSchedule is runScheduleDelete's testable core.
func deleteSchedule(store *schedule.Store, p *ui.Printer, id string) error {
	ok, err := store.Delete(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no schedule found with id %q", id)
	}
	p.Success(fmt.Sprintf("Deleted schedule %s", id))
	return nil
}

// formatScheduleTime renders a *time.Time for table/detail output, or "-" if nil.
func formatScheduleTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04 MST")
}
