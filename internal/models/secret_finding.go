package models

// SecretFinding represents a secret found during a scan.
type SecretFinding struct {
	SourceURL  string `parquet:"name=source_url, type=UTF8, encoding=PLAIN_DICTIONARY"`
	RuleID     string `parquet:"name=rule_id, type=UTF8, encoding=PLAIN_DICTIONARY"`
	SecretText string `parquet:"name=secret_text, type=UTF8, encoding=PLAIN_DICTIONARY"`
	LineNumber int    `parquet:"name=line_number, type=INT32"`
	Context    string `parquet:"name=context, type=UTF8, encoding=PLAIN_DICTIONARY"`
}
