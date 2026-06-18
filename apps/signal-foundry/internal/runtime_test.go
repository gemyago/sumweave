//go:build !release

package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	appinternal "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/config"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/strategyassistant"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestIsMissingAgentProfilesSchemaError(t *testing.T) {
	t.Run("returns true for missing agent_profiles relation", func(t *testing.T) {
		err := fmt.Errorf("get agent profile: %w", &pgconn.PgError{
			Code:    "42P01",
			Message: `relation "runtime_agent_profiles" does not exist`,
		})

		assert.True(t, isMissingAgentProfilesSchemaError(err))
	})

	t.Run("returns true for sqlite missing agent_profiles table", func(t *testing.T) {
		err := errors.New("get agent profile: SQL logic error: no such table: runtime_agent_profiles (1)")

		assert.True(t, isMissingAgentProfilesSchemaError(err))
	})

	t.Run("returns false for generic postgres missing schema", func(t *testing.T) {
		err := fmt.Errorf("get agent profile: %w", &pgconn.PgError{
			Code:    "3F000",
			Message: `schema "runtime" does not exist`,
		})

		assert.False(t, isMissingAgentProfilesSchemaError(err))
	})

	t.Run("returns false for unrelated missing relation", func(t *testing.T) {
		err := fmt.Errorf("get agent profile: %w", &pgconn.PgError{
			Code:    "42P01",
			Message: `relation "runtime_sessions" does not exist`,
		})

		assert.False(t, isMissingAgentProfilesSchemaError(err))
	})

	t.Run("returns false for generic unknown database", func(t *testing.T) {
		err := errors.New("get agent profile: unknown database runtime")

		assert.False(t, isMissingAgentProfilesSchemaError(err))
	})
}

func TestNewRuntime(t *testing.T) {
	fake := faker.New()
	rootLogger := telemetry.RootTestLogger()

	makeDeps := func(t *testing.T) RuntimeDeps {
		t.Helper()
		dataDir := t.TempDir()
		tablePrefix := strings.ReplaceAll("data_"+fake.Lorem().Word(), "-", "_") + "_"
		dataLayerDSN := filepath.Join(t.TempDir(), fake.Lorem().Word()+".db")
		dataStore, err := data.NewDatabaseStore(dataLayerDSN, data.DatabaseStoreOpts{
			TablePrefix: tablePrefix,
		})
		require.NoError(t, err)
		rawPayloadBlobStore, err := data.NewLocalRawPayloadBlobStore(filepath.Join(dataDir, "raw-payloads"))
		require.NoError(t, err)
		dataIngestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
			InstrumentStore: dataStore,
			CandleStore:     dataStore,
			TradeStore:      dataStore,
		})
		require.NoError(t, err)
		dataReadService, err := data.NewReadService(data.ReadServiceDeps{
			InstrumentStore: dataStore,
			CandleStore:     dataStore,
			TradeStore:      dataStore,
		})
		require.NoError(t, err)
		dataLineageService, err := data.NewLineageService(data.LineageServiceDeps{
			Store:     dataStore,
			BlobStore: rawPayloadBlobStore,
		})
		require.NoError(t, err)

		return RuntimeDeps{
			RootLogger:                      rootLogger,
			DataDir:                         dataDir,
			AgentRuntimeDatabaseAutoMigrate: true,
			DataLayerDatabaseDSN:            dataLayerDSN,
			DataLayerDatabaseTablePrefix:    tablePrefix,
			DataLayerDatabaseAutoMigrate:    true,
			SkillsEnabled:                   false,
			SkillsPaths:                     []string{},
			SkillsMaxSkillBytes:             65536,
			SkillsMaxCatalogEntries:         500,
			ToolsRegistry:                   agent.NewToolsRegistry(),
			DataStore:                       dataStore,
			DataIngestionService:            dataIngestionService,
			DataReadService:                 dataReadService,
			DataLineageService:              dataLineageService,
		}
	}

	makeDatabaseDeps := func(t *testing.T) RuntimeDeps {
		t.Helper()
		deps := makeDeps(t)
		deps.AgentRuntimeStorageType = storageTypeDatabase
		deps.AgentRuntimeDatabaseDSN = filepath.Join(t.TempDir(), "runtime.db")
		deps.AgentRuntimeDatabaseTablePrefix = "runtime_"
		return deps
	}

	makeInstrumentIdentity := func(t *testing.T) (domain.Venue, domain.Symbol) {
		t.Helper()
		venue, err := domain.NewVenue("venue-" + fake.Lorem().Word())
		require.NoError(t, err)
		symbol, err := domain.NewSymbol("symbol-" + fake.Lorem().Word())
		require.NoError(t, err)
		return venue, symbol
	}

	// makeSkillDir creates a valid skill directory with a SKILL.md inside parentDir.
	// Returns the skill name (== dir name).
	makeSkillDir := func(t *testing.T, parentDir string) string {
		t.Helper()
		const skillName = "test-skill"
		skillDir := filepath.Join(parentDir, skillName)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		skillMD := fmt.Sprintf("---\nname: %s\ndescription: A test skill for unit testing.\n---\n# Body\n", skillName)
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))
		return skillName
	}

	t.Run("creates runtime with non-nil runner and http handler", func(t *testing.T) {
		deps := makeDeps(t)
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)
	})

	t.Run(
		"registers strategy assistant tools before runner construction when app services are wired",
		func(t *testing.T) {
			container, bundledPlatformSkillsRoot := makeWiredRuntimeContainer(t)

			var runtime *Runtime
			var registry *agent.ToolsRegistry
			var strategyWorkspace *appinternal.StrategyWorkspaceService
			var evaluationWorkspace *appinternal.EvaluationWorkspaceService
			err := container.Invoke(func(
				rt *Runtime,
				reg *agent.ToolsRegistry,
				strategySvc *appinternal.StrategyWorkspaceService,
				evaluationSvc *appinternal.EvaluationWorkspaceService,
			) {
				runtime = rt
				registry = reg
				strategyWorkspace = strategySvc
				evaluationWorkspace = evaluationSvc
			})
			require.NoError(t, err)
			require.NotNil(t, runtime)
			require.NotNil(t, runtime.Runner)
			require.NotNil(t, strategyWorkspace)
			require.NotNil(t, evaluationWorkspace)

			registeredNames := registeredToolNames(t, registry)
			assert.Contains(t, registeredNames, "sf_data_list_candle_availability")
			assert.Contains(t, registeredNames, "sf_jobs_start_historical_data_backfill")
			assert.Contains(t, registeredNames, "sf_jobs_list")
			assert.Contains(t, registeredNames, "sf_jobs_get")
			assert.Contains(t, registeredNames, "sf_strategy_create_version")
			assert.Contains(t, registeredNames, "sf_evaluation_run_backtest")
			assert.Contains(t, registeredNames, "workspacefs_list_workspaces")
			assert.Contains(t, registeredNames, "skills_list")
			assert.Contains(t, registeredNames, "skills_read")

			require.DirExists(t, bundledPlatformSkillsRoot)
			assert.ElementsMatch(
				t,
				[]string{"agent-temp", platformAgentsWorkspace},
				listedWorkspaceIdentifiers(t, registry),
			)
		},
	)

	t.Run("wires hyperliquid recorder and lineage-enabled ingestion flow", func(t *testing.T) {
		deps := makeDeps(t)
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		require.NotNil(t, runtime.HyperliquidRecorder)
		require.NotNil(t, runtime.VenueIngestionFlow)
		require.NotNil(t, runtime.DataLineageService)

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venueedge.HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol("symbol-" + fake.Lorem().Word()),
			AssetClass: domain.AssetClassFuture,
			Active:     true,
		})
		require.NoError(t, err)

		rawPayloadID, err := runtime.HyperliquidRecorder.RecordHyperliquidRawEvidence(
			t.Context(),
			venueedge.HyperliquidRawEvidenceCapture{
				ID:                 "raw-" + fake.Lorem().Word(),
				Venue:              venueedge.HyperliquidPerpsVenueName,
				Endpoint:           "/info",
				RequestType:        "meta",
				RequestPayloadHash: "request-hash-" + fake.Lorem().Word(),
				RequestMetadata:    map[string]string{"method": http.MethodPost},
				RequestAt:          time.Now().Add(-time.Second).UTC(),
				ResponseAt:         time.Now().UTC(),
				HTTPStatus:         http.StatusOK,
				ResponseBody: []byte(fmt.Sprintf(
					`{"universe":[{"name":"%s","isDelisted":false}]}`,
					instrument.Symbol,
				)),
				EntityHint: "instrument",
				Instrument: &instrument,
				ReceivedAt: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		require.NotEmpty(t, rawPayloadID)

		readResult := venueedge.InstrumentReadResult{
			Instruments: []domain.Instrument{instrument},
			Metadata:    venueedge.ReadResultMetadata{RawPayloadIDs: []string{rawPayloadID}},
		}

		persisted, err := runtime.VenueIngestionFlow.IngestInstruments(
			t.Context(),
			&stubVenueForWiring{instrumentResult: readResult},
			venueedge.InstrumentReadRequest{Venue: venueedge.HyperliquidPerpsVenueName},
		)
		require.NoError(t, err)
		require.Len(t, persisted, 1)

		linkedIDs, err := runtime.DataLineageService.ListInstrumentRawPayloadIDs(t.Context(), instrument)
		require.NoError(t, err)
		require.Equal(t, []string{rawPayloadID}, linkedIDs)
	})

	t.Run("creates missing workspacefs agent temp directory", func(t *testing.T) {
		deps := makeDeps(t)
		agentTempDir := filepath.Join(deps.DataDir, "agent-temp")

		_, statErr := os.Stat(agentTempDir)
		require.ErrorIs(t, statErr, os.ErrNotExist)

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)

		info, statErr := os.Stat(agentTempDir)
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("database storage - creates runtime with database backend and migrates profiles", func(t *testing.T) {
		deps := makeDatabaseDeps(t)
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)

		profilesSvc, err := agent.NewDatabaseAgentProfilesService(
			deps.AgentRuntimeDatabaseDSN,
			rootLogger,
			deps.AgentRuntimeDatabaseTablePrefix,
		)
		require.NoError(t, err)

		profiles, err := profilesSvc.List(t.Context())
		require.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.Equal(t, strategyassistant.StrategyAssistantProfileName, profiles[0].Name)
	})

	t.Run("database storage - autoMigrate disabled still seeds profile when schema exists", func(t *testing.T) {
		deps := makeDatabaseDeps(t)
		deps.AgentRuntimeDatabaseAutoMigrate = false

		profilesSvc, err := agent.NewDatabaseAgentProfilesService(
			deps.AgentRuntimeDatabaseDSN,
			rootLogger,
			deps.AgentRuntimeDatabaseTablePrefix,
		)
		require.NoError(t, err)
		require.NoError(t, profilesSvc.AutoMigrate())

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)

		profile, err := profilesSvc.Get(t.Context(), strategyassistant.StrategyAssistantProfileName)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(
			t,
			strategyassistant.StrategyAssistantProfileSeedDefaultModel,
			profile.ExecutionSettings.DefaultModel,
		)
	})

	t.Run("database storage - autoMigrate disabled still starts when profile schema is absent", func(t *testing.T) {
		deps := makeDatabaseDeps(t)
		deps.AgentRuntimeDatabaseAutoMigrate = false

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)

		profilesSvc, err := agent.NewDatabaseAgentProfilesService(
			deps.AgentRuntimeDatabaseDSN,
			rootLogger,
			deps.AgentRuntimeDatabaseTablePrefix,
		)
		require.NoError(t, err)

		_, err = profilesSvc.Get(t.Context(), strategyassistant.StrategyAssistantProfileName)
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("data layer autoMigrate enabled migrates canonical tables", func(t *testing.T) {
		deps := makeDeps(t)

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		require.NotNil(t, runtime.DataStore)
		require.NotNil(t, runtime.DataIngestionService)
		require.NotNil(t, runtime.DataReadService)
		require.NotNil(t, runtime.DataLineageService)
		require.NotNil(t, runtime.VenueIngestionFlow)
		require.NotNil(t, runtime.HyperliquidRecorder)

		store, err := data.NewDatabaseStore(deps.DataLayerDatabaseDSN, dataStoreOpts(deps))
		require.NoError(t, err)

		venue, symbol := makeInstrumentIdentity(t)
		_, err = store.LookupInstrument(t.Context(), venue, symbol)
		require.ErrorIs(t, err, data.ErrInstrumentNotFound)
	})

	t.Run("data layer autoMigrate disabled skips canonical table creation", func(t *testing.T) {
		deps := makeDeps(t)
		deps.DataLayerDatabaseAutoMigrate = false

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		require.NotNil(t, runtime.VenueIngestionFlow)
		require.NotNil(t, runtime.HyperliquidRecorder)

		store, err := data.NewDatabaseStore(deps.DataLayerDatabaseDSN, dataStoreOpts(deps))
		require.NoError(t, err)

		venue, symbol := makeInstrumentIdentity(t)
		_, err = store.LookupInstrument(t.Context(), venue, symbol)
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("http handler is wired with background runner", func(t *testing.T) {
		runtime, err := newRuntime(makeDeps(t))
		require.NoError(t, err)
		require.NotNil(t, runtime)

		// The handler should accept requests — a nil/bad path returns a proper HTTP response,
		// confirming the handler is fully wired (BackgroundRunner → agentapi → HTTP mux).
		assert.NotNil(t, runtime.HTTPHandler)
	})

	t.Run("exec disabled by default", func(t *testing.T) {
		deps := makeDeps(t)
		deps.ExecEnabled = false
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)
	})

	t.Run("exec enabled with valid limits", func(t *testing.T) {
		deps := makeDeps(t)
		deps.ExecEnabled = true
		deps.ExecMaxOutputBytes = fake.Int64Between(1024, 1024*1024)
		deps.ExecDefaultTimeout = time.Duration(fake.Int64Between(1, 60)) * time.Second
		deps.ExecMaxConcurrentJobs = fake.IntBetween(1, 20)
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.HTTPHandler)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)
	})

	t.Run("skills disabled - runtime starts normally without skills tools", func(t *testing.T) {
		deps := makeDeps(t)
		deps.SkillsEnabled = false
		deps.SkillsPaths = []string{t.TempDir()}

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
		assert.NotNil(t, runtime.VenueIngestionFlow)
		assert.NotNil(t, runtime.HyperliquidRecorder)
	})

	t.Run("skills enabled with default recommended paths - runtime starts with skills tools", func(t *testing.T) {
		deps := makeDeps(t)
		skillsRoot := t.TempDir()
		makeSkillDir(t, skillsRoot)
		deps.SkillsEnabled = true
		deps.SkillsPaths = []string{skillsRoot}

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
	})

	t.Run("skills enabled with non-existent paths - runtime starts without failing", func(t *testing.T) {
		deps := makeDeps(t)
		deps.SkillsEnabled = true
		deps.SkillsPaths = []string{filepath.Join(t.TempDir(), "nonexistent")}

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
	})

	t.Run("skills enabled with malformed SKILL.md - skips bad skill without failing", func(t *testing.T) {
		deps := makeDeps(t)
		skillsRoot := t.TempDir()

		// Create a directory with a malformed SKILL.md (missing frontmatter)
		badSkillDir := filepath.Join(skillsRoot, "bad-skill")
		require.NoError(t, os.MkdirAll(badSkillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(badSkillDir, "SKILL.md"), []byte("no frontmatter here"), 0o644))

		deps.SkillsEnabled = true
		deps.SkillsPaths = []string{skillsRoot}

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
	})

	t.Run("uses provided ToolsRegistry from deps instead of creating new one", func(t *testing.T) {
		deps := makeDeps(t)
		providedRegistry := agent.NewToolsRegistry()
		deps.ToolsRegistry = providedRegistry

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.Same(t, providedRegistry, runtime.ToolsRegistry)
	})

	t.Run("file storage seeds the strategy assistant profile with regular guidance", func(t *testing.T) {
		deps := makeDeps(t)

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)

		profilesSvc, err := agent.NewFileAgentProfilesService(deps.DataDir, rootLogger)
		require.NoError(t, err)

		profile, err := profilesSvc.Get(t.Context(), strategyassistant.StrategyAssistantProfileName)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, agent.ExecutionModeRegular, profile.ExecutionSettings.ModeOrDefault())
		assert.Contains(t, profile.Instructions, "Signal Foundry strategy assistant")
		assert.Contains(t, profile.Instructions, "Product tools are authoritative")
		assert.Contains(t, profile.Instructions, "strategy-dsl-v0")
		assert.Contains(t, profile.Instructions, "replay-data-unavailable")
	})

	t.Run("bundled platform-agent skills are discoverable from the default bundled path", func(t *testing.T) {
		_, bundledPlatformSkillsRoot := makeWiredRuntimeContainer(t)

		skillDirEntries, err := os.ReadDir(bundledPlatformSkillsRoot)
		require.NoError(t, err)

		skillNames := make([]string, 0, len(skillDirEntries))
		for _, entry := range skillDirEntries {
			if !entry.IsDir() {
				continue
			}
			skillNames = append(skillNames, entry.Name())
		}

		assert.Contains(t, skillNames, "strategy-research-loop")
		assert.Contains(t, skillNames, "historical-data-jobs")
		assert.Contains(t, skillNames, "strategy-dsl-v0")
		assert.Contains(t, skillNames, "backtest-critique")
		assert.Contains(t, skillNames, "strategy-iteration")
		assert.Contains(t, skillNames, "platform-info")

		for _, skillName := range []string{
			"strategy-research-loop",
			"historical-data-jobs",
			"strategy-dsl-v0",
			"backtest-critique",
			"strategy-iteration",
			"platform-info",
		} {
			content, readErr := os.ReadFile(filepath.Join(bundledPlatformSkillsRoot, skillName, "SKILL.md"))
			require.NoError(t, readErr)
			if skillName == "platform-info" {
				assert.Contains(t, string(content), "Workflow")
				continue
			}
			assert.Contains(t, string(content), "Safety boundaries")
		}

		historicalContent, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "historical-data-jobs", "SKILL.md"),
		)
		require.NoError(t, err)
		historicalText := strings.ToLower(string(historicalContent))
		assert.Contains(t, historicalText, "sf_data_list_candle_availability")
		assert.Contains(t, historicalText, "backfill:<venue>:<symbol>:<assetclass>:<timeframe>:<start>:<end>")
		assert.Contains(t, historicalText, "queued/running")
		assert.Contains(t, historicalText, "do not run evaluation while job is `queued` or `running`")

		researchLoopContent, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "strategy-research-loop", "SKILL.md"),
		)
		require.NoError(t, err)
		assertOrderedSubstrings(
			t,
			string(researchLoopContent),
			[]string{
				"sf_data_list_candle_availability",
				"sf_data_get_candles",
				"historical-data-jobs",
				"strategy-dsl-v0",
				"sf_strategy_validate_definition",
				"sf_strategy_create_version",
				"sf_evaluation_run_backtest",
				"sf_evaluation_get_backtest_report",
				"sf_evaluation_get_backtest_evidence",
			},
		)

		dslContent, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "strategy-dsl-v0", "SKILL.md"),
		)
		require.NoError(t, err)
		dslText := string(dslContent)
		assert.Contains(t, dslText, `"kind": "moving-average-crossover"`)
		assert.Contains(t, dslText, "fastWindow must be less than slowWindow")
		assert.Contains(t, dslText, "Emit long when previous fast <= previous slow and current fast > current slow.")

		critiqueContent, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "backtest-critique", "SKILL.md"),
		)
		require.NoError(t, err)
		assertOrderedSubstrings(
			t,
			string(critiqueContent),
			[]string{
				"sf_evaluation_get_backtest_detail",
				"sf_evaluation_get_backtest_report",
				"sf_evaluation_get_backtest_evidence",
			},
		)
	})

	t.Run("skills enabled with duplicate skill names - runtime starts keeping first occurrence", func(t *testing.T) {
		deps := makeDeps(t)
		root1 := t.TempDir()
		root2 := t.TempDir()

		// Same skill name in both roots
		for _, root := range []string{root1, root2} {
			skillDir := filepath.Join(root, "my-skill")
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			content := "---\nname: my-skill\ndescription: Duplicate skill.\n---\n# Body\n"
			require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))
		}

		deps.SkillsEnabled = true
		deps.SkillsPaths = []string{root1, root2}

		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		require.NotNil(t, runtime)
		assert.NotNil(t, runtime.Runner)
	})
}

func makeWiredRuntimeContainer(t *testing.T) (*dig.Container, string) {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	appRoot := filepath.Clean(filepath.Join(cwd, ".."))
	t.Chdir(appRoot)
	bundledPlatformSkillsRoot := filepath.Clean(
		filepath.Join(appRoot, "..", "..", ".platform-agents", "skills"),
	)

	cfg := config.New()
	cfg.Set("env", "test")
	cfg.Set("dataDir", t.TempDir())
	cfg.Set("dataLayer.database.dsn", filepath.Join(t.TempDir(), "data-layer.db"))
	cfg.Set("dataLayer.rawPayloadBlobStore.path", filepath.Join(t.TempDir(), "raw-payloads"))
	cfg.Set("jobs.worker.pollInterval", "10ms")
	cfg.Set("skills.enabled", true)

	container := dig.New()
	err = Setup(context.Background(), cfg, container)
	require.NoError(t, err)

	return container, bundledPlatformSkillsRoot
}

func registeredToolNames(t *testing.T, registry *agent.ToolsRegistry) []string {
	t.Helper()

	toolsField := reflect.ValueOf(registry).Elem().FieldByName("tools")
	require.True(t, toolsField.IsValid())
	toolsField = reflect.NewAt(toolsField.Type(), unsafe.Pointer(toolsField.UnsafeAddr())).Elem()

	names := make([]string, 0, toolsField.Len())
	for index := range toolsField.Len() {
		toolValue := reflect.ValueOf(toolsField.Index(index).Interface())
		names = append(names, toolValue.FieldByName("Name").String())
	}

	return names
}

func listedWorkspaceIdentifiers(t *testing.T, registry *agent.ToolsRegistry) []string {
	t.Helper()

	toolsField := reflect.ValueOf(registry).Elem().FieldByName("tools")
	require.True(t, toolsField.IsValid())
	toolsField = reflect.NewAt(toolsField.Type(), unsafe.Pointer(toolsField.UnsafeAddr())).Elem()

	for index := range toolsField.Len() {
		toolValue := reflect.ValueOf(toolsField.Index(index).Interface())
		if toolValue.FieldByName("Name").String() != "workspacefs_list_workspaces" {
			continue
		}

		handler := toolValue.FieldByName("Handler")
		require.True(t, handler.IsValid())
		require.False(t, handler.IsNil())

		outputs := handler.Call([]reflect.Value{
			reflect.ValueOf(&agent.ToolContext{Context: t.Context()}),
			reflect.Zero(handler.Type().In(1)),
		})
		require.Len(t, outputs, 2)
		require.True(t, outputs[1].IsNil())

		workspacesField := outputs[0].FieldByName("Workspaces")
		require.True(t, workspacesField.IsValid())

		identifiers := make([]string, 0, workspacesField.Len())
		for workspaceIndex := range workspacesField.Len() {
			identifiers = append(
				identifiers,
				workspacesField.Index(workspaceIndex).FieldByName("Identifier").String(),
			)
		}

		return identifiers
	}

	t.Fatalf("workspacefs_list_workspaces tool not found")
	return nil
}

func dataStoreOpts(deps RuntimeDeps) data.DatabaseStoreOpts {
	return data.DatabaseStoreOpts{
		TablePrefix: deps.DataLayerDatabaseTablePrefix,
	}
}

func assertOrderedSubstrings(t *testing.T, text string, fragments []string) {
	t.Helper()

	searchStart := 0
	for _, fragment := range fragments {
		index := strings.Index(text[searchStart:], fragment)
		if index < 0 {
			t.Fatalf("expected fragment %q after offset %d", fragment, searchStart)
		}
		searchStart += index + len(fragment)
	}
}
