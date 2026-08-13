package appdispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
)

const postgresMessageColumnCount = 4

type singleTablePostgresSchema struct {
	wmsql.DefaultPostgreSQLSchema
}

func postgresSchema(config Config) singleTablePostgresSchema {
	return singleTablePostgresSchema{DefaultPostgreSQLSchema: wmsql.DefaultPostgreSQLSchema{
		//nolint:gocritic // Every topic intentionally uses the configured shared table.
		GenerateMessagesTableName: func(string) string { return quoteIdentifier(config.MessagesTable()) },
		GeneratePayloadType:       func(string) string { return "BYTEA" },
	}}
}

func (s singleTablePostgresSchema) SchemaInitializingQueries(
	_ wmsql.SchemaInitializingQueriesParams,
) ([]wmsql.Query, error) {
	table := s.MessagesTable("")
	return []wmsql.Query{
		{Query: `CREATE TABLE IF NOT EXISTS ` + table + ` (
			"offset" BIGSERIAL,
			uuid VARCHAR(36) NOT NULL,
			topic VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			payload BYTEA DEFAULT NULL,
			metadata JSON DEFAULT NULL,
			transaction_id xid8 NOT NULL,
			PRIMARY KEY (transaction_id, "offset")
		)`},
		{Query: `CREATE INDEX IF NOT EXISTS ` + quoteIdentifier(strings.Trim(table, `"`)+"_topic_order_idx") +
			` ON ` + table + ` (topic, transaction_id, "offset")`},
	}, nil
}

func (s singleTablePostgresSchema) InsertQuery(params wmsql.InsertQueryParams) (wmsql.Query, error) {
	markers := strings.Builder{}
	args := make([]any, 0, len(params.Msgs)*postgresMessageColumnCount)
	parameter := 1
	for index, message := range params.Msgs {
		if index > 0 {
			markers.WriteByte(',')
		}
		fmt.Fprintf(
			&markers,
			"($%d,$%d,$%d,$%d,pg_current_xact_id())",
			parameter,
			parameter+1,
			parameter+2,
			parameter+3,
		)
		parameter += postgresMessageColumnCount
		metadata, err := json.Marshal(message.Metadata)
		if err != nil {
			return wmsql.Query{}, fmt.Errorf("marshal message %s metadata: %w", message.UUID, err)
		}
		args = append(args, message.UUID, params.Topic, message.Payload, metadata)
	}
	return wmsql.Query{
		Query: `INSERT INTO ` + s.MessagesTable(params.Topic) +
			` (uuid, topic, payload, metadata, transaction_id) VALUES ` + markers.String(),
		Args: args,
	}, nil
}

func (s singleTablePostgresSchema) SelectQuery(params wmsql.SelectQueryParams) (wmsql.Query, error) {
	offsets, ok := params.OffsetsAdapter.(singleTablePostgresOffsets)
	if !ok {
		return wmsql.Query{}, errors.New("single-table postgres offsets adapter is required")
	}
	nextOffsetQuery, err := offsets.NextOffsetQuery(wmsql.NextOffsetQueryParams{
		Topic:         params.Topic,
		ConsumerGroup: params.ConsumerGroup,
	})
	if err != nil {
		return wmsql.Query{}, fmt.Errorf("create next postgres offset query: %w", err)
	}
	return wmsql.Query{
		Query: `SELECT "offset", uuid, payload, metadata, transaction_id FROM (
			WITH last_processed AS (` + nextOffsetQuery.Query + `)
			SELECT "offset", uuid, payload, metadata, transaction_id
			FROM ` + s.MessagesTable(params.Topic) + `
			WHERE topic = $1
			AND (
				(transaction_id = (SELECT last_processed_transaction_id FROM last_processed)
					AND "offset" > (SELECT offset_acked FROM last_processed))
				OR transaction_id > (SELECT last_processed_transaction_id FROM last_processed)
			)
			AND transaction_id < pg_snapshot_xmin(pg_current_snapshot())
		) AS messages
		ORDER BY transaction_id, "offset"
		LIMIT 100`,
		Args: nextOffsetQuery.Args,
	}, nil
}

func (s singleTablePostgresSchema) UnmarshalMessage(params wmsql.UnmarshalMessageParams) (wmsql.Row, error) {
	row := wmsql.Row{ExtraData: make(map[string]any)}
	var (
		uuid          string
		transactionID wmsql.XID8
	)
	if err := params.Row.Scan(&row.Offset, &uuid, &row.Payload, &row.Metadata, &transactionID); err != nil {
		return wmsql.Row{}, fmt.Errorf("scan postgres message row: %w", err)
	}
	message := wmmessage.NewMessage(uuid, row.Payload)
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &message.Metadata); err != nil {
			return wmsql.Row{}, fmt.Errorf("unmarshal postgres message metadata: %w", err)
		}
	}
	row.UUID = []byte(uuid)
	row.Msg = message
	row.ExtraData["transaction_id"] = transactionID
	return row, nil
}

type singleTablePostgresOffsets struct {
	wmsql.DefaultPostgreSQLOffsetsAdapter
}

func postgresOffsets(config Config) singleTablePostgresOffsets {
	return singleTablePostgresOffsets{DefaultPostgreSQLOffsetsAdapter: wmsql.DefaultPostgreSQLOffsetsAdapter{
		//nolint:gocritic // Every topic intentionally uses the configured shared offsets table.
		GenerateMessagesOffsetsTableName: func(string) string { return quoteIdentifier(config.OffsetsTable()) },
	}}
}

func (a singleTablePostgresOffsets) SchemaInitializingQueries(
	_ wmsql.OffsetsSchemaInitializingQueriesParams,
) ([]wmsql.Query, error) {
	return []wmsql.Query{{Query: `CREATE TABLE IF NOT EXISTS ` + a.MessagesOffsetsTable("") + ` (
		topic VARCHAR(255) NOT NULL,
		consumer_group VARCHAR(255) NOT NULL,
		offset_acked BIGINT,
		last_processed_transaction_id xid8 NOT NULL,
		PRIMARY KEY (topic, consumer_group)
	)`}}, nil
}

func (a singleTablePostgresOffsets) BeforeSubscribingQueries(
	params wmsql.BeforeSubscribingQueriesParams,
) ([]wmsql.Query, error) {
	return []wmsql.Query{{
		Query: `INSERT INTO ` + a.MessagesOffsetsTable(params.Topic) +
			` (topic, consumer_group, offset_acked, last_processed_transaction_id)` +
			` VALUES ($1, $2, 0, '0'::xid8) ON CONFLICT (topic, consumer_group) DO NOTHING`,
		Args: []any{params.Topic, params.ConsumerGroup},
	}}, nil
}

func (a singleTablePostgresOffsets) NextOffsetQuery(params wmsql.NextOffsetQueryParams) (wmsql.Query, error) {
	return wmsql.Query{
		Query: `SELECT offset_acked, last_processed_transaction_id FROM ` + a.MessagesOffsetsTable(params.Topic) +
			` WHERE topic=$1 AND consumer_group=$2 FOR UPDATE`,
		Args: []any{params.Topic, params.ConsumerGroup},
	}, nil
}

func (a singleTablePostgresOffsets) AckMessageQuery(params wmsql.AckMessageQueryParams) (wmsql.Query, error) {
	transactionID, ok := params.LastRow.ExtraData["transaction_id"]
	if !ok {
		return wmsql.Query{}, errors.New("transaction_id not found in message row")
	}
	return wmsql.Query{
		Query: `UPDATE ` + a.MessagesOffsetsTable(params.Topic) +
			` SET offset_acked=$1, last_processed_transaction_id=$2 WHERE topic=$3 AND consumer_group=$4`,
		Args: []any{params.LastRow.Offset, transactionID, params.Topic, params.ConsumerGroup},
	}, nil
}
