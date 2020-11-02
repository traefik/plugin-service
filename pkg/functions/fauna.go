package functions

import (
	"encoding/json"

	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/rs/zerolog/log"
)

func observer(result *faunadb.QueryResult) {
	if result.StatusCode/100 != 2 {
		query, _ := json.Marshal(result.Query)

		ctx := log.With().
			Str("query", string(query)).
			Int("query", result.StatusCode).
			Time("StartTime", result.StartTime).
			Time("EndTime", result.EndTime)

		for name, values := range result.Headers {
			ctx.Strs("RESPONSE_HEADER_"+name, values)
		}

		logger := ctx.Logger()
		logger.Error().Msg("faunaDB call")
	}
}
