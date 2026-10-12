package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/sqls-server/sqls/internal/formatter"
	"github.com/sqls-server/sqls/internal/lsp"
)

func (s *Server) handleTextDocumentFormatting(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params lsp.DocumentFormattingParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	f, ok := s.files[params.TextDocument.URI]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", params.TextDocument.URI)
	}

	textEdits, err := formatter.Format(f.Text, params, s.getConfig())
	if err != nil {
		return nil, err
	}
	if len(textEdits) > 0 {
		return textEdits, nil
	}
	return nil, nil
}

func (s *Server) handleTextDocumentRangeFormatting(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params lsp.DocumentRangeFormattingParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	f, ok := s.files[params.TextDocument.URI]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", params.TextDocument.URI)
	}

	start, err := lsp.ByteOffset(f.Text, params.Range.Start)
	if err != nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: err.Error()}
	}
	end, err := lsp.ByteOffset(f.Text, params.Range.End)
	if err != nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: err.Error()}
	}
	if end < start {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "range end precedes start"}
	}
	edits, err := formatter.Format(f.Text[start:end], lsp.DocumentFormattingParams{Options: params.Options}, s.getConfig())
	if err != nil {
		return nil, err
	}
	if len(edits) > 0 {
		edits[0].Range = lsp.Range{Start: lsp.PositionAt(f.Text, start), End: lsp.PositionAt(f.Text, end)}
		return edits, nil
	}
	return nil, nil
}
