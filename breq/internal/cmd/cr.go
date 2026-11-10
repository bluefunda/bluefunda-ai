package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
	"github.com/bluefunda/bluerequests/internal/tui"
	"github.com/bluefunda/bluerequests/internal/ui"
)

// ─── cr ──────────────────────────────────────────────────────────────────────

var crCmd = &cobra.Command{
	Use:   "cr",
	Short: "Change request commands",
}

// ─── cr list ─────────────────────────────────────────────────────────────────

var (
	crListProject  string
	crListDesc     string
	crListStatus   string
	crListType     string
	crListSeverity string
	crListAssignee string
	crListArchived bool
)

var crListCmd = &cobra.Command{
	Use:   "list",
	Short: "List change requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.ListChangeRequests(ctx, &pb.ListChangeRequestsRequest{
			ProjectId:       crListProject,
			Description:     crListDesc,
			Status:          crListStatus,
			RequestType:     crListType,
			Severity:        crListSeverity,
			Assignee:        crListAssignee,
			IncludeArchived: crListArchived,
		})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.ListChangeRequests(ctx, &pb.ListChangeRequestsRequest{
					ProjectId:       crListProject,
					Description:     crListDesc,
					Status:          crListStatus,
					RequestType:     crListType,
					Severity:        crListSeverity,
					Assignee:        crListAssignee,
					IncludeArchived: crListArchived,
				})
			}
			if err != nil {
				return err
			}
		}

		rows := make([][]string, 0, len(resp.ChangeRequests))
		for _, cr := range resp.ChangeRequests {
			if cr == nil || cr.Id == "" || cr.Id == "<nil>" {
				continue
			}
			rows = append(rows, []string{
				cr.Id,
				cr.Description,
				ui.StatusColor(cr.Status),
				ui.SeverityColor(cr.Severity),
				cr.RequestOwner,
				cr.CreatedAt,
			})
		}
		p.Table([]string{"ID", "DESCRIPTION", "STATUS", "SEVERITY", "OWNER", "CREATED"}, rows)
		return nil
	},
}

// ─── cr get ──────────────────────────────────────────────────────────────────

var crGetCmd = &cobra.Command{
	Use:               "get <id>",
	Short:             "Get a change request by ID",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.GetChangeRequest(ctx, &pb.GetChangeRequestRequest{Id: args[0]})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.GetChangeRequest(ctx, &pb.GetChangeRequestRequest{Id: args[0]})
			}
			if err != nil {
				return err
			}
		}

		cr := resp.ChangeRequest
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", cr.Id},
				{"description", cr.Description},
				{"status", ui.StatusColor(cr.Status)},
				{"request_type", cr.RequestType},
				{"severity", ui.SeverityColor(cr.Severity)},
				{"owner", cr.RequestOwner},
				{"assignee", cr.Assignee},
				{"project_id", cr.ProjectId},
				{"archive", boolStr(cr.Archive)},
				{"created_at", cr.CreatedAt},
				{"updated_at", cr.UpdatedAt},
			},
		)
		return nil
	},
}

// ─── cr create ───────────────────────────────────────────────────────────────

var (
	crCreateDesc     string
	crCreateProject  string
	crCreateType     string
	crCreateSeverity string
	crCreateAssignee string
)

var crCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new change request",
	RunE: func(cmd *cobra.Command, args []string) error {
		if crCreateDesc == "" {
			return fmt.Errorf("--description is required")
		}

		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		req := &pb.CreateChangeRequestRequest{
			Description: crCreateDesc,
			ProjectId:   crCreateProject,
			RequestType: crCreateType,
			Severity:    crCreateSeverity,
			Assignee:    crCreateAssignee,
		}

		resp, err := conn.Client.CreateChangeRequest(ctx, req)
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.CreateChangeRequest(ctx, req)
			}
			if err != nil {
				return err
			}
		}

		cr := resp.ChangeRequest
		p.Success(fmt.Sprintf("Change request created: %s", cr.Id))
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", cr.Id},
				{"description", cr.Description},
				{"status", cr.Status},
				{"severity", cr.Severity},
				{"project_id", cr.ProjectId},
			},
		)
		return nil
	},
}

// ─── cr update ───────────────────────────────────────────────────────────────

var (
	crUpdateDesc     string
	crUpdateStatus   string
	crUpdateType     string
	crUpdateSeverity string
	crUpdateAssignee string
)

var crUpdateCmd = &cobra.Command{
	Use:               "update <id>",
	Short:             "Update a change request",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		req := &pb.UpdateChangeRequestRequest{
			Id:          args[0],
			Description: crUpdateDesc,
			Status:      crUpdateStatus,
			RequestType: crUpdateType,
			Severity:    crUpdateSeverity,
			Assignee:    crUpdateAssignee,
		}

		resp, err := conn.Client.UpdateChangeRequest(ctx, req)
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.UpdateChangeRequest(ctx, req)
			}
			if err != nil {
				return err
			}
		}

		cr := resp.ChangeRequest
		p.Success(fmt.Sprintf("Change request %s updated", cr.Id))
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", cr.Id},
				{"description", cr.Description},
				{"status", cr.Status},
				{"severity", cr.Severity},
				{"assignee", cr.Assignee},
				{"updated_at", cr.UpdatedAt},
			},
		)
		return nil
	},
}

// ─── cr delete ───────────────────────────────────────────────────────────────

var crDeleteCmd = &cobra.Command{
	Use:               "delete <id>",
	Short:             "Delete (archive) a change request",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		_, err = conn.Client.DeleteChangeRequest(ctx, &pb.DeleteChangeRequestRequest{Id: args[0]})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				_, err = conn.Client.DeleteChangeRequest(ctx, &pb.DeleteChangeRequestRequest{Id: args[0]})
			}
			if err != nil {
				return err
			}
		}

		p.Success(fmt.Sprintf("Change request %s deleted", args[0]))
		return nil
	},
}

// ─── cr stage ────────────────────────────────────────────────────────────────

var crStageValue string

var crStageCmd = &cobra.Command{
	Use:               "stage <id>",
	Short:             "Update the stage of a change request",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		if crStageValue == "" {
			return fmt.Errorf("--stage is required")
		}

		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		req := &pb.UpdateChangeRequestStageRequest{Id: args[0], Stage: crStageValue}

		resp, err := conn.Client.UpdateChangeRequestStage(ctx, req)
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.UpdateChangeRequestStage(ctx, req)
			}
			if err != nil {
				return err
			}
		}

		cr := resp.ChangeRequest
		p.Success(fmt.Sprintf("Stage updated for change request %s", cr.Id))
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", cr.Id},
				{"status", cr.Status},
				{"updated_at", cr.UpdatedAt},
			},
		)
		return nil
	},
}

// ─── cr comment ──────────────────────────────────────────────────────────────

var crCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Comment commands for a change request",
}

var crCommentListCmd = &cobra.Command{
	Use:               "list <cr-id>",
	Short:             "List comments on a change request",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.ListComments(ctx, &pb.ListCommentsRequest{ChangeRequestId: args[0]})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.ListComments(ctx, &pb.ListCommentsRequest{ChangeRequestId: args[0]})
			}
			if err != nil {
				return err
			}
		}

		rows := make([][]string, 0, len(resp.Comments))
		for _, c := range resp.Comments {
			rows = append(rows, []string{c.Id, c.CreatedBy, c.Message, c.CreatedAt})
		}
		p.Table([]string{"ID", "AUTHOR", "MESSAGE", "CREATED"}, rows)
		return nil
	},
}

var crCommentAddMsg string

var crCommentAddCmd = &cobra.Command{
	Use:               "add <cr-id>",
	Short:             "Add a comment to a change request",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCRID,
	RunE: func(cmd *cobra.Command, args []string) error {
		if crCommentAddMsg == "" {
			return fmt.Errorf("--message is required")
		}

		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		req := &pb.AddCommentRequest{ChangeRequestId: args[0], Message: crCommentAddMsg}

		resp, err := conn.Client.AddComment(ctx, req)
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.AddComment(ctx, req)
			}
			if err != nil {
				return err
			}
		}

		c := resp.Comment
		p.Success(fmt.Sprintf("Comment added: %s", c.Id))
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", c.Id},
				{"change_request_id", c.ChangeRequestId},
				{"message", c.Message},
				{"created_at", c.CreatedAt},
			},
		)
		return nil
	},
}

var crCommentUpdateMsg string

var crCommentUpdateCmd = &cobra.Command{
	Use:   "update <comment-id>",
	Short: "Update a comment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if crCommentUpdateMsg == "" {
			return fmt.Errorf("--message is required")
		}

		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		req := &pb.UpdateCommentRequest{Id: args[0], Message: crCommentUpdateMsg}

		resp, err := conn.Client.UpdateComment(ctx, req)
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				resp, err = conn.Client.UpdateComment(ctx, req)
			}
			if err != nil {
				return err
			}
		}

		c := resp.Comment
		p.Success(fmt.Sprintf("Comment %s updated", c.Id))
		p.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"id", c.Id},
				{"message", c.Message},
				{"updated_at", c.UpdatedAt},
			},
		)
		return nil
	},
}

var crCommentDeleteCmd = &cobra.Command{
	Use:   "delete <comment-id>",
	Short: "Delete a comment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		p := printer(cfg)
		ctx, cancel := trmgrpc.ContextWithTimeout()
		defer cancel()

		_, err = conn.Client.DeleteComment(ctx, &pb.DeleteCommentRequest{Id: args[0]})
		if err != nil {
			if trmgrpc.IsAuthError(err) {
				if reAuthErr := reAuthenticate(cfg, p); reAuthErr != nil {
					return reAuthErr
				}
				_, err = conn.Client.DeleteComment(ctx, &pb.DeleteCommentRequest{Id: args[0]})
			}
			if err != nil {
				return err
			}
		}

		p.Success(fmt.Sprintf("Comment %s deleted", args[0]))
		return nil
	},
}

// ─── cr watch ────────────────────────────────────────────────────────────────

var crWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Interactive TUI dashboard for change requests",
	Long: `Opens an interactive terminal dashboard for browsing and inspecting
change requests. Navigate with ↑↓ or j/k, press / to filter,
Enter to drill into a CR, and q to quit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, _, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		return tui.Run(conn)
	},
}

// ─── init ────────────────────────────────────────────────────────────────────

func init() {
	// cr list flags
	crListCmd.Flags().StringVar(&crListProject, "project", "", "Filter by project ID")
	crListCmd.Flags().StringVar(&crListDesc, "description", "", "Filter by description (partial match)")
	crListCmd.Flags().StringVar(&crListStatus, "status", "", "Filter by status")
	crListCmd.Flags().StringVar(&crListType, "type", "", "Filter by request type")
	crListCmd.Flags().StringVar(&crListSeverity, "severity", "", "Filter by severity")
	crListCmd.Flags().StringVar(&crListAssignee, "assignee", "", "Filter by assignee")
	crListCmd.Flags().BoolVar(&crListArchived, "archived", false, "Include archived change requests")

	// cr create flags
	crCreateCmd.Flags().StringVar(&crCreateDesc, "description", "", "Change request description (required)")
	crCreateCmd.Flags().StringVar(&crCreateProject, "project", "", "Project ID")
	crCreateCmd.Flags().StringVar(&crCreateType, "type", "", "Request type")
	crCreateCmd.Flags().StringVar(&crCreateSeverity, "severity", "", "Severity level")
	crCreateCmd.Flags().StringVar(&crCreateAssignee, "assignee", "", "Assignee username")

	// cr update flags
	crUpdateCmd.Flags().StringVar(&crUpdateDesc, "description", "", "New description")
	crUpdateCmd.Flags().StringVar(&crUpdateStatus, "status", "", "New status")
	crUpdateCmd.Flags().StringVar(&crUpdateType, "type", "", "New request type")
	crUpdateCmd.Flags().StringVar(&crUpdateSeverity, "severity", "", "New severity")
	crUpdateCmd.Flags().StringVar(&crUpdateAssignee, "assignee", "", "New assignee")

	// cr stage flags
	crStageCmd.Flags().StringVar(&crStageValue, "stage", "", "Stage to set (required)")

	// cr comment add/update flags
	crCommentAddCmd.Flags().StringVar(&crCommentAddMsg, "message", "", "Comment message (required)")
	crCommentUpdateCmd.Flags().StringVar(&crCommentUpdateMsg, "message", "", "New message (required)")

	// flag value completions
	for _, cmd := range []*cobra.Command{crListCmd, crUpdateCmd} {
		_ = cmd.RegisterFlagCompletionFunc("status", staticValues("open", "in_progress", "resolved", "closed"))
	}
	for _, cmd := range []*cobra.Command{crListCmd, crCreateCmd, crUpdateCmd} {
		_ = cmd.RegisterFlagCompletionFunc("severity", staticValues("low", "medium", "high", "critical"))
		_ = cmd.RegisterFlagCompletionFunc("type", staticValues("change", "release", "incident", "problem"))
	}
	_ = crStageCmd.RegisterFlagCompletionFunc("stage", staticValues("dev", "qa", "prod"))

	// wire comment subcommands
	crCommentCmd.AddCommand(crCommentListCmd, crCommentAddCmd, crCommentUpdateCmd, crCommentDeleteCmd)

	// wire cr subcommands
	crCmd.AddCommand(crListCmd, crGetCmd, crCreateCmd, crUpdateCmd, crDeleteCmd, crStageCmd, crCommentCmd, crWatchCmd)
}
