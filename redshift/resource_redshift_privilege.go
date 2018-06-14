package redshift

//https://docs.aws.amazon.com/redshift/latest/dg/r_GRANT.html
//https://docs.aws.amazon.com/redshift/latest/dg/r_REVOKE.html

/*
GRANT { { SELECT | INSERT | UPDATE | DELETE | REFERENCES } [,...] | ALL [ PRIVILEGES ] }
ON { ALL TABLES IN SCHEMA schema_name [, ...] }
TO { username [ WITH GRANT OPTION ] | GROUP group_name | PUBLIC } [, ...]



Permission model is limited:
	Grant
	Then do ALTER DEFAULT privileges


How to read:
	SELECT HAS_TABLE_PRIVILEGE('user1', 'table3', 'select');
 */