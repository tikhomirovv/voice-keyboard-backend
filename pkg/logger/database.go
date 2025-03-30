package logger

import "context"

const DatabaseQueryLabel = "ex_database_query_label"

func ExtractDatabaseQueryLabelFromContext(ctx context.Context) string {
	var label string
	ctxVal := ctx.Value(DatabaseQueryLabel)
	if ctxVal != nil {
		ctxValStr, ok := ctxVal.(string)
		if ok {
			label = ctxValStr
		}
	}
	return label
}
