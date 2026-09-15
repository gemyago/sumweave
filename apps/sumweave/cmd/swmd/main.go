package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/models"
	"github.com/gemyago/sumweave/apps/sumweave/internal/swmdclient"
	"github.com/spf13/cobra"
)

const (
	defaultJobWaitInterval = time.Second
	defaultJobWaitTimeout  = time.Minute
	tokenStdinNoOptValue   = "\x00"
)

type commandDeps struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	resolver   *swmdclient.Resolver
	httpClient *http.Client
}

type presenceOnlyFlag struct{}

func (*presenceOnlyFlag) Set(value string) error {
	if value != tokenStdinNoOptValue {
		return errors.New("--token-stdin does not accept a value")
	}
	return nil
}

func (*presenceOnlyFlag) String() string { return "false" }

func (*presenceOnlyFlag) Type() string { return "presence-only" }

func setupCommands() *cobra.Command {
	return newRootCmd(commandDeps{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		resolver:   swmdclient.NewResolver(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	})
}

func newRootCmd(deps commandDeps) *cobra.Command {
	var configPath string
	var baseURL string
	cmd := &cobra.Command{Use: "swmd", Short: "Sumweave direct HTTP client", SilenceUsage: true}
	cmd.SetIn(deps.stdin)
	cmd.SetOut(deps.stdout)
	cmd.SetErr(deps.stderr)
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to local configuration")
	cmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Sumweave API base URL")
	loadClient := func() (*swmdclient.Client, error) {
		config, _, err := deps.resolver.Resolve(baseURL, configPath)
		if err != nil {
			return nil, err
		}
		return swmdclient.NewClient(config, deps.httpClient)
	}
	cmd.AddCommand(
		newAuthCmd(deps, &configPath, &baseURL, loadClient),
		newTenantCmd(loadClient),
		newAccountCmd(loadClient),
		newTransactionCmd(loadClient),
		newConnectionCmd(loadClient),
		newClassificationCmd(loadClient),
		newTransferCmd(loadClient),
		newJobCmd(loadClient),
	)
	return cmd
}

type clientLoader func() (*swmdclient.Client, error)

//nolint:goconst,govet // Cobra command names remain readable beside their flag wiring.
func newAuthCmd(deps commandDeps, configPath, baseURL *string, loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage local authentication configuration"}
	configure := &cobra.Command{
		Use:   "configure",
		Short: "Persist a base URL and token read from stdin",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("base-url") || !cmd.Flags().Changed("token-stdin") {
				return errors.New("--base-url and --token-stdin are required")
			}
			token, err := swmdclient.ReadToken(cmd.InOrStdin())
			if err != nil {
				return err
			}
			selectedPath, err := deps.resolver.ConfigPath(*configPath)
			if err != nil {
				return err
			}
			return swmdclient.WriteConfig(selectedPath, swmdclient.Config{BaseURL: *baseURL, APIToken: token})
		},
	}
	configure.Flags().StringVar(baseURL, "base-url", "", "Sumweave API base URL")
	configure.Flags().Var(&presenceOnlyFlag{}, "token-stdin", "Read API token from standard input")
	configure.Flags().Lookup("token-stdin").NoOptDefVal = tokenStdinNoOptValue

	var offline bool
	status := &cobra.Command{
		Use:   "status",
		Short: "Show redacted local authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, _, err := deps.resolver.Resolve(*baseURL, *configPath)
			if err != nil {
				return err
			}
			result := struct {
				BaseURL  string           `json:"baseUrl"`
				APIToken string           `json:"apiToken"`
				User     *models.UserInfo `json:"user,omitempty"`
			}{BaseURL: config.BaseURL, APIToken: "redacted"}
			if !offline {
				client, err := loadClient()
				if err != nil {
					return err
				}
				result.User, err = client.CurrentUser(cmd.Context())
				if err != nil {
					return err
				}
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	status.Flags().BoolVar(&offline, "offline", false, "Do not call the API")

	clearCommand := &cobra.Command{
		Use:   "clear",
		Short: "Remove persisted authentication configuration",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			selectedPath, err := deps.resolver.ConfigPath(*configPath)
			if err != nil {
				return err
			}
			return swmdclient.ClearConfig(selectedPath)
		},
	}
	cmd.AddCommand(configure, status, clearCommand)
	return cmd
}

//nolint:goconst // Cobra command names remain readable beside their route wiring.
func newTenantCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "tenant", Short: "Read tenants"}
	cmd.AddCommand(&cobra.Command{
		Use:  "list",
		Args: cobra.NoArgs,
		RunE: getCommand(loadClient, "/api/v1/finance/tenants", nil, &models.FinanceTenantListResponse{}),
	})
	return cmd
}

//nolint:goconst // Cobra command names remain readable beside their route wiring.
func newAccountCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Read accounts and provider data"}
	var tenant, account, snapshot string
	var includeHidden bool
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		query := url.Values{}
		if cmd.Flags().Changed("include-hidden") {
			query.Set("includeHidden", strconv.FormatBool(includeHidden))
		}
		return getAndWrite(cmd, loadClient, financeRoute(tenant, "accounts"), query, &models.FinanceAccountsResponse{})
	}}
	list.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")
	list.Flags().BoolVar(&includeHidden, "include-hidden", false, "Include hidden accounts")
	_ = list.MarkFlagRequired("tenant")
	get := &cobra.Command{Use: "get", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return getAndWrite(cmd, loadClient, financeRoute(tenant, "accounts", account), nil, &models.FinanceAccount{})
	}}
	addTenantAndIDFlags(get, &tenant, "account", &account)
	providerList := &cobra.Command{
		Use:  "provider-data-list",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndWrite(
				cmd, loadClient, financeRoute(tenant, "accounts", account, "provider-snapshots"),
				nil, &models.FinanceProviderSnapshotListResponse{},
			)
		},
	}
	addTenantAndIDFlags(providerList, &tenant, "account", &account)
	providerGet := &cobra.Command{
		Use:  "provider-data-get",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndWrite(
				cmd, loadClient, financeRoute(tenant, "accounts", account, "provider-snapshots", snapshot),
				nil, &models.FinanceProviderSnapshot{},
			)
		},
	}
	addTenantAndIDFlags(providerGet, &tenant, "account", &account)
	providerGet.Flags().StringVar(&snapshot, "snapshot", "", "Provider snapshot ID")
	_ = providerGet.MarkFlagRequired("snapshot")
	cmd.AddCommand(list, get, providerList, providerGet)
	return cmd
}

//nolint:gocognit,goconst,govet // The command owns its complete public filter surface.
func newTransactionCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "transaction", Short: "Read transactions and provider data"}
	var tenant, account, transaction, snapshot, source, status, kind, startDate, endDate, sort string
	var includeHidden bool
	var limit, offset int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 200 {
			return errors.New("--limit must be between 1 and 200")
		}
		if offset < 0 {
			return errors.New("--offset must not be negative")
		}
		for _, value := range []string{startDate, endDate} {
			if value != "" {
				if _, err := parseTimestamp(value); err != nil {
					return err
				}
			}
		}
		query := url.Values{"limit": {strconv.FormatInt(limit, 10)}, "offset": {strconv.FormatInt(offset, 10)}}
		for key, value := range map[string]string{"accountId": account, "source": source, "status": status, "kind": kind, "startDate": startDate, "endDate": endDate, "sort": sort} {
			if value != "" {
				query.Set(key, value)
			}
		}
		if cmd.Flags().Changed("include-hidden") {
			query.Set("includeHidden", strconv.FormatBool(includeHidden))
		}
		client, err := loadClient()
		if err != nil {
			return err
		}
		all := &models.FinanceTransactionsResponse{Items: make([]*models.FinanceTransaction, 0)}
		for {
			page := &models.FinanceTransactionsResponse{}
			if err := client.Get(cmd.Context(), financeRoute(tenant, "transactions"), query, page); err != nil {
				return err
			}
			all.Items = append(all.Items, page.Items...)
			if len(page.Items) < int(limit) {
				break
			}
			offset += limit
			query.Set("offset", strconv.FormatInt(offset, 10))
		}
		return writeJSON(cmd.OutOrStdout(), all)
	}}
	list.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")
	_ = list.MarkFlagRequired("tenant")
	list.Flags().StringVar(&account, "account", "", "Account ID")
	list.Flags().StringVar(&source, "source", "", "Transaction source")
	list.Flags().StringVar(&status, "status", "", "Transaction status")
	list.Flags().StringVar(&kind, "kind", "", "Transaction kind")
	list.Flags().StringVar(&startDate, "start-date", "", "Inclusive RFC 3339 timestamp")
	list.Flags().StringVar(&endDate, "end-date", "", "Exclusive RFC 3339 timestamp")
	list.Flags().StringVar(&sort, "sort", "", "Effective timestamp order")
	list.Flags().BoolVar(&includeHidden, "include-hidden", false, "Include hidden transactions")
	list.Flags().Int64Var(&limit, "limit", 100, "Page size (1-200)")
	list.Flags().Int64Var(&offset, "offset", 0, "Initial page offset")
	get := &cobra.Command{Use: "get", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return getAndWrite(
			cmd, loadClient, financeRoute(tenant, "transactions", transaction),
			nil, &models.FinanceTransaction{},
		)
	}}
	addTenantAndIDFlags(get, &tenant, "transaction", &transaction)
	providerList := &cobra.Command{
		Use:  "provider-data-list",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndWrite(
				cmd, loadClient, financeRoute(tenant, "transactions", transaction, "provider-snapshots"),
				nil, &models.FinanceProviderSnapshotListResponse{},
			)
		},
	}
	addTenantAndIDFlags(providerList, &tenant, "transaction", &transaction)
	providerGet := &cobra.Command{
		Use:  "provider-data-get",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndWrite(
				cmd, loadClient, financeRoute(tenant, "transactions", transaction, "provider-snapshots", snapshot),
				nil, &models.FinanceProviderSnapshot{},
			)
		},
	}
	addTenantAndIDFlags(providerGet, &tenant, "transaction", &transaction)
	providerGet.Flags().StringVar(&snapshot, "snapshot", "", "Provider snapshot ID")
	_ = providerGet.MarkFlagRequired("snapshot")
	cmd.AddCommand(list, get, providerList, providerGet)
	return cmd
}

//nolint:goconst // Cobra command names remain readable beside their route wiring.
func newConnectionCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "connection", Short: "Read connections and submit synchronization"}
	var tenant, connection, reason, windowStart, windowEnd, idempotencyKey string
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return getAndWrite(
			cmd, loadClient, financeRoute(tenant, "connections"), nil,
			&models.FinanceConnectionsResponse{},
		)
	}}
	list.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")
	_ = list.MarkFlagRequired("tenant")
	sync := &cobra.Command{Use: "sync", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		request := models.FinanceConnectionSyncRequest{Reason: reason}
		if windowStart != "" {
			value, err := parseTimestamp(windowStart)
			if err != nil {
				return err
			}
			request.WindowStart = &value
		}
		if windowEnd != "" {
			value, err := parseTimestamp(windowEnd)
			if err != nil {
				return err
			}
			request.WindowEnd = &value
		}
		return postAndWrite(
			cmd, loadClient, financeRoute(tenant, "connections", connection, "sync"),
			request, &models.FinanceFxSyncResponse{}, idempotencyKey,
		)
	}}
	addTenantAndIDFlags(sync, &tenant, "connection", &connection)
	sync.Flags().StringVar(&reason, "reason", "", "Synchronization reason")
	sync.Flags().StringVar(&windowStart, "window-start", "", "RFC 3339 window start")
	sync.Flags().StringVar(&windowEnd, "window-end", "", "RFC 3339 window end")
	sync.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	cmd.AddCommand(list, sync)
	return cmd
}

//nolint:dupl,goconst // These separate commands intentionally retain generated request types.
func newClassificationCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "classification", Short: "Submit transaction classification"}
	var tenant, rangeStart, rangeEnd, idempotencyKey string
	run := &cobra.Command{Use: "run", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		start, err := parseTimestamp(rangeStart)
		if err != nil {
			return err
		}
		end, err := parseTimestamp(rangeEnd)
		if err != nil {
			return err
		}
		return postAndWrite(
			cmd, loadClient, financeRoute(tenant, "transactions", "classify"),
			models.FinanceTransactionClassificationRequest{RangeStart: start, RangeEndExclusive: end},
			&models.FinanceClassificationJobResponse{}, idempotencyKey,
		)
	}}
	run.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")
	run.Flags().StringVar(&rangeStart, "range-start", "", "Inclusive RFC 3339 timestamp")
	run.Flags().StringVar(&rangeEnd, "range-end-exclusive", "", "Exclusive RFC 3339 timestamp")
	run.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	_ = run.MarkFlagRequired("tenant")
	_ = run.MarkFlagRequired("range-start")
	_ = run.MarkFlagRequired("range-end-exclusive")
	cmd.AddCommand(run)
	return cmd
}

//nolint:dupl,goconst // These separate commands intentionally retain generated request types.
func newTransferCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "transfer", Short: "Submit transfer matching"}
	var tenant, rangeStart, rangeEnd, idempotencyKey string
	match := &cobra.Command{Use: "match", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		start, err := parseTimestamp(rangeStart)
		if err != nil {
			return err
		}
		end, err := parseTimestamp(rangeEnd)
		if err != nil {
			return err
		}
		return postAndWrite(
			cmd, loadClient, financeRoute(tenant, "transactions", "match-transfers"),
			models.FinanceTransferMatchingRequest{RangeStart: start, RangeEndExclusive: end},
			&models.FinanceTransferMatchingJobResponse{}, idempotencyKey,
		)
	}}
	match.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")
	match.Flags().StringVar(&rangeStart, "range-start", "", "Inclusive RFC 3339 timestamp")
	match.Flags().StringVar(&rangeEnd, "range-end-exclusive", "", "Exclusive RFC 3339 timestamp")
	match.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	_ = match.MarkFlagRequired("tenant")
	_ = match.MarkFlagRequired("range-start")
	_ = match.MarkFlagRequired("range-end-exclusive")
	cmd.AddCommand(match)
	return cmd
}

//nolint:goconst,govet // Cobra command names remain readable beside their route wiring.
func newJobCmd(loadClient clientLoader) *cobra.Command {
	cmd := &cobra.Command{Use: "job", Short: "Read and wait for jobs"}
	var job, cursor string
	var statuses, types, sources []string
	var limit int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 100 {
			return errors.New("--limit must be between 1 and 100")
		}
		query := url.Values{"limit": {strconv.FormatInt(limit, 10)}}
		for key, values := range map[string][]string{"status": statuses, "jobType": types, "source": sources} {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		return getAndWrite(cmd, loadClient, "/api/v1/jobs", query, &models.JobListResponse{})
	}}
	list.Flags().StringSliceVar(&statuses, "status", nil, "Job statuses")
	list.Flags().StringSliceVar(&types, "job-type", nil, "Job types")
	list.Flags().StringSliceVar(&sources, "source", nil, "Requester sources")
	list.Flags().Int64Var(&limit, "limit", 25, "Page size (1-100)")
	list.Flags().StringVar(&cursor, "cursor", "", "Page cursor")
	get := &cobra.Command{Use: "get", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return getAndWrite(cmd, loadClient, jobRoute(job), nil, &models.JobDetailResponse{})
	}}
	get.Flags().StringVar(&job, "job", "", "Job ID")
	_ = get.MarkFlagRequired("job")
	var interval, timeout time.Duration
	wait := &cobra.Command{Use: "wait", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := loadClient()
		if err != nil {
			return err
		}
		output, waitErr := client.WaitForJob(cmd.Context(), job, swmdclient.WaitOptions{
			Interval: interval,
			Timeout:  timeout,
		})
		if output != nil {
			if err := writeJSON(cmd.OutOrStdout(), output); err != nil {
				return err
			}
		}
		return waitErr
	}}
	wait.Flags().StringVar(&job, "job", "", "Expected job ID")
	wait.Flags().DurationVar(&interval, "interval", defaultJobWaitInterval, "Polling interval")
	wait.Flags().DurationVar(&timeout, "timeout", defaultJobWaitTimeout, "Overall wait timeout")
	_ = wait.MarkFlagRequired("job")
	cmd.AddCommand(list, get, wait)
	return cmd
}

func getCommand(
	loadClient clientLoader,
	endpoint string,
	query url.Values,
	output any,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		return getAndWrite(cmd, loadClient, endpoint, query, output)
	}
}

//nolint:govet // Error scopes are intentionally limited to individual HTTP steps.
func getAndWrite(cmd *cobra.Command, loadClient clientLoader, endpoint string, query url.Values, output any) error {
	client, err := loadClient()
	if err != nil {
		return err
	}
	if err := client.Get(cmd.Context(), endpoint, query, output); err != nil {
		return err
	}
	return writeJSON(cmd.OutOrStdout(), output)
}

//nolint:govet // Error scopes are intentionally limited to individual HTTP steps.
func postAndWrite(
	cmd *cobra.Command,
	loadClient clientLoader,
	endpoint string,
	requestBody, output any,
	idempotencyKey string,
) error {
	client, err := loadClient()
	if err != nil {
		return err
	}
	if err := client.Post(cmd.Context(), endpoint, requestBody, output, idempotencyKey); err != nil {
		return err
	}
	return writeJSON(cmd.OutOrStdout(), output)
}

func addTenantAndIDFlags(cmd *cobra.Command, tenant *string, idName string, id *string) {
	cmd.Flags().StringVar(tenant, "tenant", "", "Tenant ID")
	cmd.Flags().StringVar(id, idName, "", strings.ToUpper(idName[:1])+idName[1:]+" ID")
	_ = cmd.MarkFlagRequired("tenant")
	_ = cmd.MarkFlagRequired(idName)
}

func financeRoute(tenant string, suffix ...string) string {
	parts := append([]string{"api", "v1", "finance", "tenants", tenant}, suffix...)
	return "/" + strings.Join(escapeParts(parts), "/")
}

func jobRoute(job string) string { return "/api/v1/jobs/" + url.PathEscape(job) }

func escapeParts(parts []string) []string {
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return parts
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp must be RFC 3339: %w", err)
	}
	return parsed, nil
}

func writeJSON(output io.Writer, value any) error {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}

func main() { // coverage-ignore
	if err := setupCommands().ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
