package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "LLM model operations",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available LLM models",
	RunE:  runModelList,
}

func init() {
	modelCmd.AddCommand(modelListCmd)
}

func runModelList(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetLLMModels(ctx, &pb.GetLLMModelsRequest{})
	if err != nil {
		return fmt.Errorf("get models: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"NAME", "ID", "OWNED_BY"}
	rows := make([][]string, 0, len(resp.GetModels()))
	for _, m := range resp.GetModels() {
		rows = append(rows, []string{
			m.GetName(),
			fmt.Sprintf("%d", m.GetModelId()),
			m.GetOwnedBy(),
		})
	}
	p.Table(headers, rows)
	return nil
}
