package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	commandIncludeInSkillAnnotation   = "swmd.includeInSkill"
	commandExcludeFromSkillAnnotation = "swmd.excludeFromSkill"
	commandHelpName                   = "help"
	commandCompletionName             = "completion"
	skillCommandName                  = "skill"
	commandAnnotationEnabled          = "true"
	skillRootSubcommandLevel          = 2
	skillFrontmatterName              = "swmd"
	skillFrontmatterDescription       = "Use the swmd CLI for Sumweave finance reads, sync/classification/transfer triggers, and job inspection or waiting; do not use it to create, change, or clear credentials."
)

const skillFixedBody = `# swmd CLI Skill

Use ` + "`swmd`" + ` as the HTTP-only client for Sumweave finance and job operations.

## Operating rules

- Treat stdout from operational commands as one complete JSON value.
- Treat stderr as diagnostics and a nonzero exit status as failure.
- Pass ` + "`--tenant`" + ` explicitly whenever a command requires tenant scope.
- Preserve supplied RFC 3339 offsets in timestamp flags.
- Reuse the same ` + "`--idempotency-key`" + ` when retrying the same trigger.
- After a trigger returns a job ID, use ` + "`swmd job wait --job \"$JOB_ID\"`" + ` to wait for its terminal result.
- Use ` + "`swmd auth status`" + ` only to inspect redacted authentication status.
- Do not run ` + "`swmd auth configure`" + ` or ` + "`swmd auth clear`" + `; credential setup is an operator responsibility.
`

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     skillCommandName,
		Short:   "Generate an agent skill for swmd",
		Long:    "Generate a Markdown skill from the live swmd command tree.",
		Example: "swmd skill > .agents/skills/swmd/SKILL.md",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeSkillGuide(cmd.Root(), cmd.OutOrStdout())
		},
		Annotations: map[string]string{commandExcludeFromSkillAnnotation: commandAnnotationEnabled},
	}
	return cmd
}

func writeSkillGuide(root *cobra.Command, out io.Writer) error {
	if root == nil {
		return errors.New("missing root command")
	}

	var guide strings.Builder
	guide.WriteString(renderSkillFrontmatter())
	guide.WriteString(skillFixedBody)
	guide.WriteString("\n")
	guide.WriteString("## `" + root.CommandPath() + "`\n\n")
	guide.WriteString(commandDescription(root) + "\n\n")
	guide.WriteString(renderCommandUsage(root) + "\n")
	renderSkillFlags(&guide, "Global Flags", root.PersistentFlags())
	renderSkillCommand(&guide, root, skillRootSubcommandLevel)

	if _, err := out.Write([]byte(guide.String())); err != nil {
		return fmt.Errorf("write skill output: %w", err)
	}
	return nil
}

func renderSkillFrontmatter() string {
	return "---\nname: " + strconv.Quote(skillFrontmatterName) + "\ndescription: " +
		strconv.Quote(skillFrontmatterDescription) + "\n---\n\n"
}

func commandDescription(cmd *cobra.Command) string {
	description := strings.TrimSpace(cmd.Long)
	if description == "" {
		description = strings.TrimSpace(cmd.Short)
	}
	return description
}

func renderCommandUsage(cmd *cobra.Command) string {
	usage := strings.TrimSpace(cmd.UseLine())
	if usage == "" {
		return ""
	}
	return fmt.Sprintf("Usage: `%s`", usage)
}

func renderSkillCommand(out *strings.Builder, cmd *cobra.Command, level int) {
	for _, child := range cmd.Commands() {
		renderChild := shouldRenderInSkill(child)
		if renderChild {
			out.WriteString(strings.Repeat("#", level+1) + " `" + child.CommandPath() + "`\n\n")
			out.WriteString(commandDescription(child) + "\n\n")
			out.WriteString(renderCommandUsage(child) + "\n\n")
			renderSkillFlags(out, "Flags", child.NonInheritedFlags())
			renderSkillExample(out, child)
		}

		nextLevel := level
		if renderChild {
			nextLevel++
		}
		renderSkillCommand(out, child, nextLevel)
	}
}

func renderSkillExample(out *strings.Builder, cmd *cobra.Command) {
	example := strings.TrimSpace(cmd.Example)
	if example == "" {
		return
	}
	out.WriteString("Examples:\n\n```bash\n" + example + "\n```\n\n")
}

func renderSkillFlags(out *strings.Builder, heading string, flags *pflag.FlagSet) {
	if flags == nil || !containsRenderableFlags(flags) {
		return
	}
	out.WriteString("### " + heading + "\n\n")
	required, optional := splitRenderableFlags(flags)
	renderFlagGroup(out, "Required Parameters", required)
	renderFlagGroup(out, "Optional Parameters", optional)
}

func containsRenderableFlags(flags *pflag.FlagSet) bool {
	renderable := false
	flags.VisitAll(func(flag *pflag.Flag) { renderable = renderable || shouldRenderFlag(flag) })
	return renderable
}

func shouldRenderFlag(flag *pflag.Flag) bool {
	return flag != nil && !flag.Hidden && flag.Name != commandHelpName
}

func splitRenderableFlags(flags *pflag.FlagSet) ([]*pflag.Flag, []*pflag.Flag) {
	var required, optional []*pflag.Flag
	flags.VisitAll(func(flag *pflag.Flag) {
		if !shouldRenderFlag(flag) {
			return
		}
		if isRequiredFlag(flag) {
			required = append(required, flag)
			return
		}
		optional = append(optional, flag)
	})
	return required, optional
}

func isRequiredFlag(flag *pflag.Flag) bool {
	if flag == nil || flag.Annotations == nil {
		return false
	}
	values, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]
	return ok && len(values) > 0 && values[0] == "true"
}

func renderFlag(flag *pflag.Flag) string {
	name := "`--" + flag.Name + "`"
	if flag.Shorthand != "" {
		name += ", `-" + flag.Shorthand + "`"
	}
	return name + ": " + strings.TrimSpace(flag.Usage)
}

func renderFlagGroup(out *strings.Builder, heading string, flags []*pflag.Flag) {
	if len(flags) == 0 {
		return
	}
	out.WriteString("#### " + heading + "\n\n")
	for _, flag := range flags {
		out.WriteString("- " + renderFlag(flag) + "\n")
	}
	out.WriteString("\n")
}

func shouldRenderInSkill(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Hidden || isBuiltInSkillCommand(cmd) {
		return false
	}
	for current := cmd; current != nil; current = current.Parent() {
		if current.Hidden || isBuiltInSkillCommand(current) {
			return false
		}
		if current != cmd && hasCommandAnnotation(current, commandExcludeFromSkillAnnotation) {
			return hasCommandAnnotation(cmd, commandIncludeInSkillAnnotation)
		}
	}
	if hasCommandAnnotation(cmd, commandIncludeInSkillAnnotation) {
		return true
	}
	if hasCommandAnnotation(cmd, commandExcludeFromSkillAnnotation) {
		return false
	}
	return true
}

func isBuiltInSkillCommand(cmd *cobra.Command) bool {
	return cmd.Name() == commandHelpName || cmd.Name() == commandCompletionName
}

func hasCommandAnnotation(cmd *cobra.Command, annotation string) bool {
	return cmd != nil && cmd.Annotations != nil && cmd.Annotations[annotation] == commandAnnotationEnabled
}
