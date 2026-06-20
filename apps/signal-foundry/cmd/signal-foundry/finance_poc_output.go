package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeFinancePOCEnvelope(out io.Writer, outputFile string, jsonOutput bool, envelope financePOCEnvelope) error {
	payload, err := marshalFinancePOCPayload(envelope)
	if err != nil {
		return err
	}
	writeErr := writeFinancePOCPayload(out, outputFile, jsonOutput, payload)
	if writeErr != nil {
		return writeErr
	}
	if jsonOutput {
		return nil
	}

	_, err = io.WriteString(
		out,
		fmt.Sprintf(
			"provider: %s\noperation: %s\nfetched_at: %s\n",
			envelope.Provider,
			envelope.Operation,
			envelope.FetchedAt,
		),
	)
	return err
}

func writeFinancePOCJSONPayload(
	out io.Writer,
	outputFile string,
	jsonOutput bool,
	payloadValue any,
	textOutput string,
) error {
	payload, err := marshalFinancePOCPayload(payloadValue)
	if err != nil {
		return err
	}
	writeErr := writeFinancePOCPayload(out, outputFile, jsonOutput, payload)
	if writeErr != nil {
		return writeErr
	}
	if jsonOutput {
		return nil
	}
	_, err = io.WriteString(out, textOutput)
	return err
}

func marshalFinancePOCPayload(payloadValue any) ([]byte, error) {
	payload, err := json.MarshalIndent(payloadValue, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal finance-poc output: %w", err)
	}
	return append(payload, '\n'), nil
}

func writeFinancePOCPayload(out io.Writer, outputFile string, jsonOutput bool, payload []byte) error {
	if strings.TrimSpace(outputFile) != "" {
		if writeErr := writeFinancePOCFile(outputFile, payload); writeErr != nil {
			return writeErr
		}
	}
	if !jsonOutput {
		return nil
	}
	_, err := out.Write(payload)
	return err
}

func writeFinancePOCFile(outputFile string, payload []byte) error {
	parentDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return fmt.Errorf("create finance-poc output directory: %w", err)
	}
	if err := os.WriteFile(outputFile, payload, 0o600); err != nil {
		return fmt.Errorf("write finance-poc output file: %w", err)
	}
	return nil
}

func writeFinancePOCProgressf(out io.Writer, format string, args ...any) {
	if out == nil {
		return
	}
	_, _ = fmt.Fprintf(out, format+"\n", args...)
}
