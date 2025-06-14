package models

// SecretFinding represents a secret found during a scan.
type SecretFinding struct {
	SourceURL  string `parquet:"source_url,plain_dictionary,utf8"`
	RuleID     string `parquet:"rule_id,plain_dictionary,utf8"`
	SecretText string `parquet:"secret_text,plain_dictionary,utf8"`
	LineNumber int    `parquet:"line_number,int32"`
	Context    string `parquet:"context,plain_dictionary,utf8"`
}
