package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestSkillCommand(t *testing.T) {
	makeRoot := func(output io.Writer) *cobra.Command {
		root := newRootCmd(commandDeps{stdin: strings.NewReader(""), stdout: output, stderr: io.Discard})
		root.SetArgs([]string{"skill"})
		return root
	}

	t.Run("renders an offline deterministic guide without credential side effects", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "credentials.json")
		var first, second bytes.Buffer
		root := makeRoot(&first)
		root.SetArgs([]string{"--config", configPath, "skill"})

		require.NoError(t, root.Execute())
		require.NoFileExists(t, configPath)
		require.NotEmpty(t, first.String())
		require.NoError(t, makeRoot(&second).Execute())
		require.Equal(t, first.String(), second.String())
	})

	t.Run("uses fixed frontmatter and operating rules", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, makeRoot(&output).Execute())
		guide := output.String()
		require.Equal(t, "swmd", skillFrontmatterName)
		require.Equal(
			t,
			"Use the swmd CLI for Sumweave finance reads, sync/classification/transfer triggers, and job inspection or waiting; do not use it to create, change, or clear credentials.",
			skillFrontmatterDescription,
		)
		expectedFrontmatter := "---\nname: \"swmd\"\ndescription: \"Use the swmd CLI for Sumweave finance reads, " +
			"sync/classification/transfer triggers, and job inspection or waiting; do not use it to create, change, or clear credentials.\"\n---\n\n"
		expectedBody := "# swmd CLI Skill\n\n" +
			"Use `swmd` as the HTTP-only client for Sumweave finance and job operations.\n\n" +
			"## Operating rules\n\n" +
			"- Treat stdout from operational commands as one complete JSON value.\n" +
			"- Treat stderr as diagnostics and a nonzero exit status as failure.\n" +
			"- Pass `--tenant` explicitly whenever a command requires tenant scope.\n" +
			"- Preserve supplied RFC 3339 offsets in timestamp flags.\n" +
			"- Reuse the same `--idempotency-key` when retrying the same trigger.\n" +
			"- After a trigger returns a job ID, use `swmd job wait --job \"$JOB_ID\"` to wait for its terminal result.\n" +
			"- Use `swmd auth status` only to inspect redacted authentication status.\n" +
			"- Do not run `swmd auth configure` or `swmd auth clear`; credential setup is an operator responsibility.\n"
		require.True(t, strings.HasPrefix(guide, expectedFrontmatter+expectedBody+"\n## `swmd`\n"))
		require.Equal(t, 1, strings.Count(strings.SplitN(guide, "---\n\n", 2)[0], "\nname: "))
		require.Equal(t, 1, strings.Count(strings.SplitN(guide, "---\n\n", 2)[0], "\ndescription: "))
	})

	t.Run("projects global flags and included commands only", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, makeRoot(&output).Execute())
		guide := output.String()
		require.Contains(t, guide, "### Global Flags")
		require.Contains(t, guide, "`--base-url`: Sumweave API base URL")
		require.Contains(t, guide, "`--config`: Path to local configuration")
		require.Contains(t, guide, "### `swmd auth status`")
		require.NotContains(t, guide, "### `swmd auth configure`")
		require.NotContains(t, guide, "### `swmd auth clear`")
		require.NotContains(t, guide, "### `swmd skill`")
		require.NotContains(t, guide, "### `swmd completion`")
		require.NotContains(t, guide, "token-stdin")
		require.NotContains(t, guide, "`--help`")
		require.Equal(t, 1, strings.Count(guide, "`--base-url`"))
		require.Equal(t, 1, strings.Count(guide, "`--config`"))
		require.Contains(t, guide, "```bash\nswmd account list --tenant \"$TENANT_ID\"\n```")
	})

	t.Run("keeps required and optional flags in pflag order", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, makeRoot(&output).Execute())
		guide := output.String()
		section := strings.Split(strings.Split(guide, "#### `swmd classification run`\n\n")[1], "### `swmd connection`")[0]
		require.Contains(t, section, "#### Required Parameters")
		require.Contains(t, section, "#### Optional Parameters")
		require.Less(
			t,
			strings.Index(section, "#### Required Parameters"),
			strings.Index(section, "#### Optional Parameters"),
		)
		require.Less(t, strings.Index(section, "--range-end-exclusive"), strings.Index(section, "--range-start"))
		require.Less(t, strings.Index(section, "--range-start"), strings.Index(section, "--tenant"))
	})

	t.Run("returns useful renderer errors", func(t *testing.T) {
		require.EqualError(t, writeSkillGuide(nil, io.Discard), "missing root command")
		file, err := os.Create(filepath.Join(t.TempDir(), "closed-output"))
		require.NoError(t, err)
		require.NoError(t, file.Close())
		err = writeSkillGuide(&cobra.Command{Use: "swmd"}, file)
		require.Error(t, err)
		require.ErrorContains(t, err, "write skill output")
		require.ErrorIs(t, err, os.ErrClosed)
	})

	t.Run("uses exact command metadata", func(t *testing.T) {
		root := makeRoot(io.Discard)
		type commandMetadata struct {
			short   string
			long    string
			example string
		}
		expected := map[string]commandMetadata{
			"auth status": {
				"Show redacted authentication and current-user status",
				"",
				"swmd auth status\nswmd auth status --offline",
			},
			"tenant list": {
				"List accessible finance tenants",
				"",
				"swmd tenant list",
			},
			"account list": {
				"List accounts for a tenant",
				"",
				`swmd account list --tenant "$TENANT_ID"`,
			},
			"account get": {
				"Get an account",
				"",
				`swmd account get --tenant "$TENANT_ID" --account "$ACCOUNT_ID"`,
			},
			"account provider-data-list": {
				"List stored provider snapshots for an account",
				"",
				`swmd account provider-data-list --tenant "$TENANT_ID" --account "$ACCOUNT_ID"`,
			},
			"account provider-data-get": {
				"Get a stored provider snapshot for an account",
				"",
				`swmd account provider-data-get --tenant "$TENANT_ID" --account "$ACCOUNT_ID" --snapshot "$SNAPSHOT_ID"`,
			},
			"transaction list": {
				"List transactions for a tenant",
				"List transactions beginning at the requested offset and continue through bounded API pages until all matching transactions are returned.",
				`swmd transaction list --tenant "$TENANT_ID" --limit 100 --offset 0`,
			},
			"transaction get": {
				"Get a transaction",
				"",
				`swmd transaction get --tenant "$TENANT_ID" --transaction "$TRANSACTION_ID"`,
			},
			"transaction provider-data-list": {
				"List stored provider snapshots for a transaction",
				"",
				`swmd transaction provider-data-list --tenant "$TENANT_ID" --transaction "$TRANSACTION_ID"`,
			},
			"transaction provider-data-get": {
				"Get a stored provider snapshot for a transaction",
				"",
				`swmd transaction provider-data-get --tenant "$TENANT_ID" --transaction "$TRANSACTION_ID" --snapshot "$SNAPSHOT_ID"`,
			},
			"connection list": {
				"List connections for a tenant",
				"",
				`swmd connection list --tenant "$TENANT_ID"`,
			},
			"connection sync": {
				"Submit connection synchronization",
				"Submit connection synchronization and return its job reference. Reuse the same idempotency key when retrying the same request.",
				`swmd connection sync --tenant "$TENANT_ID" --connection "$CONNECTION_ID" --idempotency-key "$IDEMPOTENCY_KEY"`,
			},
			"classification run": {
				"Submit transaction classification for a time range",
				"Submit classification for the exact RFC 3339 range and return its job reference.",
				`swmd classification run --tenant "$TENANT_ID" --range-start "2026-09-01T00:00:00+02:00" --range-end-exclusive "2026-10-01T00:00:00+02:00"`,
			},
			"transfer match": {
				"Submit transfer matching for a time range",
				"Submit transfer matching for the exact RFC 3339 range and return its job reference.",
				`swmd transfer match --tenant "$TENANT_ID" --range-start "2026-09-01T00:00:00+02:00" --range-end-exclusive "2026-10-01T00:00:00+02:00"`,
			},
			"job list": {
				"List requester-visible jobs",
				"",
				"swmd job list --status queued,running --limit 25",
			},
			"job get": {
				"Get a job",
				"",
				`swmd job get --job "$JOB_ID"`,
			},
			"job wait": {
				"Wait for a job to reach a terminal state",
				"Poll an expected job until success, failure, not-found after the materialization grace period, or timeout. Write terminal job JSON and exit nonzero for failure or timeout.",
				`swmd job wait --job "$JOB_ID" --interval 1s --timeout 1m`,
			},
		}
		lookup := func(path string) *cobra.Command {
			command := root
			for _, name := range strings.Split(path, " ") {
				var found *cobra.Command
				for _, child := range command.Commands() {
					if child.Name() == name {
						found = child
						break
					}
				}
				require.NotNil(t, found, path)
				command = found
			}
			return command
		}
		for path, metadata := range expected {
			command := lookup(path)
			require.Equal(t, metadata.short, command.Short, path)
			require.Equal(t, metadata.long, command.Long, path)
			require.Equal(t, metadata.example, command.Example, path)
			description := metadata.short
			if metadata.long != "" {
				description = metadata.long
			}
			var guide strings.Builder
			require.NoError(t, writeSkillGuide(root, &guide))
			require.Contains(t, guide.String(), "`swmd "+path+"`\n\n"+description+"\n\n", path)
		}
		skill := lookup("skill")
		require.Equal(t, "Generate an agent skill for swmd", skill.Short)
		require.Equal(t, "Generate a Markdown skill from the live swmd command tree.", skill.Long)
		require.Equal(t, "swmd skill > .agents/skills/swmd/SKILL.md", skill.Example)
		require.Equal(t, "swmd is the HTTP-only command-line client for Sumweave finance data and jobs.", root.Long)
	})

	t.Run("honors annotations and cannot include hidden or built-in commands", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		excluded := &cobra.Command{
			Use:         "excluded",
			Annotations: map[string]string{commandExcludeFromSkillAnnotation: commandAnnotationEnabled},
		}
		included := &cobra.Command{
			Use:         "included",
			Annotations: map[string]string{commandIncludeInSkillAnnotation: commandAnnotationEnabled},
		}
		hidden := &cobra.Command{
			Use:         "hidden",
			Hidden:      true,
			Annotations: map[string]string{commandIncludeInSkillAnnotation: commandAnnotationEnabled},
		}
		help := &cobra.Command{
			Use:         "help",
			Annotations: map[string]string{commandIncludeInSkillAnnotation: commandAnnotationEnabled},
		}
		excluded.AddCommand(included)
		hidden.AddCommand(&cobra.Command{
			Use:         "child",
			Annotations: map[string]string{commandIncludeInSkillAnnotation: commandAnnotationEnabled},
		})
		root.AddCommand(excluded, hidden, help)

		require.False(t, shouldRenderInSkill(excluded))
		require.True(t, shouldRenderInSkill(included))
		require.False(t, shouldRenderInSkill(hidden))
		require.False(t, shouldRenderInSkill(hidden.Commands()[0]))
		require.False(t, shouldRenderInSkill(help))
		var rendered strings.Builder
		renderSkillCommand(&rendered, root, skillRootSubcommandLevel)
		require.Contains(t, rendered.String(), "### `root excluded included`")
		require.NotContains(t, rendered.String(), "hidden")
		require.NotContains(t, rendered.String(), "help")
	})

	t.Run("filters hidden help flags", func(t *testing.T) {
		flags := pflag.NewFlagSet("flags", pflag.ContinueOnError)
		flags.String("visible", "", "visible flag")
		flags.String("hidden", "", "hidden flag")
		flags.Bool("help", false, "help flag")
		require.NoError(t, flags.MarkHidden("hidden"))
		var rendered strings.Builder
		renderSkillFlags(&rendered, "Flags", flags)
		require.Contains(t, rendered.String(), "visible flag")
		require.NotContains(t, rendered.String(), "hidden flag")
		require.NotContains(t, rendered.String(), "help flag")
	})
}
