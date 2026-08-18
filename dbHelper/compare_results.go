package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
)

// ---------------------- COMPARE CONFIG ----------------------

var (
	// compareResultSortRows controls whether rows are sorted before comparison.
	compareResultSortRows = true

	// compareResultCaseSensitive controls whether text comparison is case-sensitive.
	compareResultCaseSensitive = true

	// compareResultRunConcurrently controls whether multiple SQL pairs are checked
	// concurrently. It has no effect in file mode, because file mode always compares
	// exactly the two files configured below.
	compareResultRunConcurrently = false

	// The following three variables are used only when
	// compareResultRunConcurrently is true. Workers repeatedly choose a random
	// pair from compareResultSQLPairs instead of checking every pair just once.
	compareResultConcurrentWorkerCount = 4

	// A zero duration disables the time limit.
	compareResultConcurrentRunDuration = 1 * time.Minute

	// Zero disables the per-pair limit. When this value is positive, every pair
	// can be scheduled at most this many times. If both limits are enabled, new
	// work stops when the duration elapses or every pair reaches its run limit,
	// whichever happens first. Comparisons already in progress are allowed to
	// finish.
	compareResultConcurrentRunsPerPair = 0

	// compareResultReadFromFiles selects the source of the two result sets:
	// true reads the two files below; false executes compareResultSQLPairs.
	compareResultReadFromFiles = false

	compareResultExpectedFile = ""
	compareResultActualFile   = ""

	// You can override it with parameter in CLI
	// For example: go run . --task compare-results --addr
	compareResultDBConfig = dbConfig{
		address: defaultAddress,
		port:    defaultPort,
		user:    defaultUser,
		dbName:  defaultDBName,
		params:  []string{},
	}

	// Add as many pairs as needed. setupSQLs are executed on the same database
	// session as query, immediately before query is executed.
	//
	// Example:
	// {
	//     name: "partial ordered index for topn",
	//     expected: compareSQL{
	//         setupSQLs: []string{`set tidb_opt_partial_ordered_index_for_topn="disable"`},
	//         query:     "select c0, c1 from t1",
	//     },
	//     actual: compareSQL{
	//         setupSQLs: []string{`set tidb_opt_partial_ordered_index_for_topn="cost"`},
	//         query:     "select c0, c1 from t1",
	//     },
	// },
	//
	// {
	// 	name: "",
	// 	expected: compareSQL{
	// 		setupSQLs: []string{
	// 			"",
	// 		},
	// 		query: "",
	// 	},
	// 	actual: compareSQL{
	// 		setupSQLs: []string{
	// 			"",
	// 		},
	// 		query: "",
	// 	},
	// },
	compareResultSQLPairs = []compareSQLPair{
		// --------------------- TPCH ---------------------
		// {
		// 	name: "tpch1",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select PS_PARTKEY, PS_SUPPKEY % 20000 as col0, length(PS_COMMENT) as col1, PS_COMMENT as col2 from test.partsupp where length(ps_comment) > 190) select t1.PS_PARTKEY, t1.col0, t1.col1, t1.col2 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select PS_PARTKEY, PS_SUPPKEY % 20000 as col0, length(PS_COMMENT) as col1, PS_COMMENT as col2 from test.partsupp where length(ps_comment) > 190) select t1.PS_PARTKEY, t1.col0, t1.col1, t1.col2 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// },
		// {
		// 	name: "tpch2",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select PS_PARTKEY, substring(PS_COMMENT, 1, 30) as col0, substring(PS_COMMENT, 20, 30) as col1 from test.partsupp) select t1.PS_PARTKEY, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select PS_PARTKEY, substring(PS_COMMENT, 1, 30) as col0, substring(PS_COMMENT, 20, 30) as col1 from test.partsupp) select t1.PS_PARTKEY, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// },
		// {
		// 	name: "tpch3",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select ps_partkey, ps_suppkey, (ps_supplycost + ps_partkey) * 13 as col0, (ps_supplycost + ps_suppkey) * 13 as col1 from test.partsupp) select t1.ps_partkey, t1.ps_suppkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select ps_partkey, ps_suppkey, (ps_supplycost + ps_partkey) * 13 as col0, (ps_supplycost + ps_suppkey) * 13 as col1 from test.partsupp) select t1.ps_partkey, t1.ps_suppkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// },
		// {
		// 	name: "tpch4",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select o_orderkey, date_add(o_orderdate, interval o_orderkey%10000000 hour) as col0, date_add(o_orderdate, interval o_orderkey%20000000 hour) as col1 from test.orders) select t1.o_orderkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select o_orderkey, date_add(o_orderdate, interval o_orderkey%10000000 hour) as col0, date_add(o_orderdate, interval o_orderkey%20000000 hour) as col1 from test.orders) select t1.o_orderkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1",
		// 	},
		// },
		// {
		// 	name: "tpch5",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select ps_partkey, substring(ps_comment, 1, 20) as col0, substring(ps_comment, 2, 4) as col1 from test.partsupp), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1 from test.customer) select t3.c_custkey, t3.col1, t6.col2 from (select t1.c_custkey, t1.col1 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.ps_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.ps_partkey",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select ps_partkey, substring(ps_comment, 1, 20) as col0, substring(ps_comment, 2, 4) as col1 from test.partsupp), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1 from test.customer) select t3.c_custkey, t3.col1, t6.col2 from (select t1.c_custkey, t1.col1 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.ps_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.ps_partkey",
		// 	},
		// },
		// {
		// 	name: "tpch6",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select ps_partkey, substring(ps_comment, 1, 20) as col0, substring(ps_comment, 2, 4) as col1, substring(ps_comment, 5, 4) as col2 from test.partsupp), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1, substring(c_address, 5, 4) as col2 from test.customer) select t7.col0, t7.col1, t8.c_custkey from (select t3.c_custkey as col0, t3.col1 as col1, t6.col2 as col2 from (select t1.c_custkey, t2.col2 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.ps_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.ps_partkey) as t7 join cte2 as t8 on t7.col1 = t8.col2",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select ps_partkey, substring(ps_comment, 1, 20) as col0, substring(ps_comment, 2, 4) as col1, substring(ps_comment, 5, 4) as col2 from test.partsupp), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1, substring(c_address, 5, 4) as col2 from test.customer) select t7.col0, t7.col1, t8.c_custkey from (select t3.c_custkey as col0, t3.col1 as col1, t6.col2 as col2 from (select t1.c_custkey, t2.col2 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.ps_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.ps_partkey) as t7 join cte2 as t8 on t7.col1 = t8.col2",
		// 	},
		// },
		// {
		// 	name: "tpch7",
		// 	expected: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
		// 		},
		// 		query: "with cte1 as (select o_orderkey + 1 as col0, o_custkey as col1 from test.orders), cte2 as (select col0 + 1 as col0, col0 + 2 as col1 from cte1 union all select col0 + col1 as col0, col1 + 1 as col1 from cte1) select * from cte2 t1 join cte2 t2 on t1.col0 = t2.col1",
		// 	},
		// 	actual: compareSQL{
		// 		setupSQLs: []string{
		// 			"set tidb_enforce_mpp=1",
		// 			"set tidb_opt_enable_mpp_shared_cte_execution=on",
		// 			fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
		// 		},
		// 		query: "with cte1 as (select o_orderkey + 1 as col0, o_custkey as col1 from test.orders), cte2 as (select col0 + 1 as col0, col0 + 2 as col1 from cte1 union all select col0 + col1 as col0, col1 + 1 as col1 from cte1) select * from cte2 t1 join cte2 t2 on t1.col0 = t2.col1",
		// 	},
		// },
		// --------------------- TPCDS ---------------------
		{
			name: "tpcds1",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH customer_total_return AS (SELECT sr_customer_sk AS ctr_customer_sk, sr_store_sk AS ctr_store_sk, Sum(sr_return_amt) AS ctr_total_return FROM store_returns, date_dim WHERE sr_returned_date_sk = d_date_sk AND d_year = 2001 GROUP BY sr_customer_sk, sr_store_sk) SELECT c_customer_id FROM customer_total_return ctr1, store, customer WHERE  ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_store_sk = ctr2.ctr_store_sk) AND s_store_sk = ctr1.ctr_store_sk AND s_state = 'TN' AND ctr1.ctr_customer_sk = c_customer_sk ORDER  BY c_customer_id LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH customer_total_return AS (SELECT sr_customer_sk AS ctr_customer_sk, sr_store_sk AS ctr_store_sk, Sum(sr_return_amt) AS ctr_total_return FROM store_returns, date_dim WHERE sr_returned_date_sk = d_date_sk AND d_year = 2001 GROUP BY sr_customer_sk, sr_store_sk) SELECT c_customer_id FROM customer_total_return ctr1, store, customer WHERE  ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_store_sk = ctr2.ctr_store_sk) AND s_store_sk = ctr1.ctr_store_sk AND s_state = 'TN' AND ctr1.ctr_customer_sk = c_customer_sk ORDER  BY c_customer_id LIMIT 100",
			},
		},
		{
			name: "tpcds2",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "with wscs as (select ws_sold_date_sk sold_date_sk ,ws_ext_sales_price sales_price from web_sales union all select cs_sold_date_sk sold_date_sk ,cs_ext_sales_price sales_price from catalog_sales), wswscs as (select d_week_seq, sum(case when (d_day_name='Sunday') then sales_price else null end) sun_sales, sum(case when (d_day_name='Monday') then sales_price else null end) mon_sales, sum(case when (d_day_name='Tuesday') then sales_price else null end) tue_sales, sum(case when (d_day_name='Wednesday') then sales_price else null end) wed_sales, sum(case when (d_day_name='Thursday') then sales_price else null end) thu_sales, sum(case when (d_day_name='Friday') then sales_price else null end) fri_sales, sum(case when (d_day_name='Saturday') then sales_price else null end) sat_sales from wscs ,date_dim where d_date_sk = sold_date_sk group by d_week_seq) select d_week_seq1 ,round(sun_sales1/sun_sales2,2) ,round(mon_sales1/mon_sales2,2) ,round(tue_sales1/tue_sales2,2) ,round(wed_sales1/wed_sales2,2) ,round(thu_sales1/thu_sales2,2) ,round(fri_sales1/fri_sales2,2) ,round(sat_sales1/sat_sales2,2) from (select wswscs.d_week_seq d_week_seq1 ,sun_sales sun_sales1 ,mon_sales mon_sales1 ,tue_sales tue_sales1 ,wed_sales wed_sales1 ,thu_sales thu_sales1 ,fri_sales fri_sales1 ,sat_sales sat_sales1 from wswscs,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001) y, (select wswscs.d_week_seq d_week_seq2 ,sun_sales sun_sales2 ,mon_sales mon_sales2 ,tue_sales tue_sales2 ,wed_sales wed_sales2 ,thu_sales thu_sales2 ,fri_sales fri_sales2 ,sat_sales sat_sales2 from wswscs ,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001+1) z where d_week_seq1=d_week_seq2-53 order by d_week_seq1",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "with wscs as (select ws_sold_date_sk sold_date_sk ,ws_ext_sales_price sales_price from web_sales union all select cs_sold_date_sk sold_date_sk ,cs_ext_sales_price sales_price from catalog_sales), wswscs as (select d_week_seq, sum(case when (d_day_name='Sunday') then sales_price else null end) sun_sales, sum(case when (d_day_name='Monday') then sales_price else null end) mon_sales, sum(case when (d_day_name='Tuesday') then sales_price else null end) tue_sales, sum(case when (d_day_name='Wednesday') then sales_price else null end) wed_sales, sum(case when (d_day_name='Thursday') then sales_price else null end) thu_sales, sum(case when (d_day_name='Friday') then sales_price else null end) fri_sales, sum(case when (d_day_name='Saturday') then sales_price else null end) sat_sales from wscs ,date_dim where d_date_sk = sold_date_sk group by d_week_seq) select d_week_seq1 ,round(sun_sales1/sun_sales2,2) ,round(mon_sales1/mon_sales2,2) ,round(tue_sales1/tue_sales2,2) ,round(wed_sales1/wed_sales2,2) ,round(thu_sales1/thu_sales2,2) ,round(fri_sales1/fri_sales2,2) ,round(sat_sales1/sat_sales2,2) from (select wswscs.d_week_seq d_week_seq1 ,sun_sales sun_sales1 ,mon_sales mon_sales1 ,tue_sales tue_sales1 ,wed_sales wed_sales1 ,thu_sales thu_sales1 ,fri_sales fri_sales1 ,sat_sales sat_sales1 from wswscs,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001) y, (select wswscs.d_week_seq d_week_seq2 ,sun_sales sun_sales2 ,mon_sales mon_sales2 ,tue_sales tue_sales2 ,wed_sales wed_sales2 ,thu_sales thu_sales2 ,fri_sales fri_sales2 ,sat_sales sat_sales2 from wswscs ,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001+1) z where d_week_seq1=d_week_seq2-53 order by d_week_seq1",
			},
		},
		{
			name: "tpcds3",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(( ( ss_ext_list_price - ss_ext_wholesale_cost - ss_ext_discount_amt ) + ss_ext_sales_price ) / 2) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( cs_ext_list_price - cs_ext_wholesale_cost - cs_ext_discount_amt ) + cs_ext_sales_price ) / 2 )) year_total FROM customer, catalog_sales, date_dim WHERE c_customer_sk = cs_bill_customer_sk AND cs_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( ws_ext_list_price - ws_ext_wholesale_cost - ws_ext_discount_amt ) + ws_ext_sales_price ) / 2 )) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_c_firstyear, year_total t_c_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_c_secyear.customer_id AND t_s_firstyear.customer_id = t_c_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_c_firstyear.dyear = 2001 AND t_c_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_c_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE NULL END AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE NULL END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(( ( ss_ext_list_price - ss_ext_wholesale_cost - ss_ext_discount_amt ) + ss_ext_sales_price ) / 2) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( cs_ext_list_price - cs_ext_wholesale_cost - cs_ext_discount_amt ) + cs_ext_sales_price ) / 2 )) year_total FROM customer, catalog_sales, date_dim WHERE c_customer_sk = cs_bill_customer_sk AND cs_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( ws_ext_list_price - ws_ext_wholesale_cost - ws_ext_discount_amt ) + ws_ext_sales_price ) / 2 )) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_c_firstyear, year_total t_c_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_c_secyear.customer_id AND t_s_firstyear.customer_id = t_c_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_c_firstyear.dyear = 2001 AND t_c_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_c_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE NULL END AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE NULL END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag LIMIT 100",
			},
		},
		{
			name: "tpcds4",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ss_ext_list_price - ss_ext_discount_amt) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ws_ext_list_price - ws_ext_discount_amt) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE 0.0 END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE 0.0 END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ss_ext_list_price - ss_ext_discount_amt) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ws_ext_list_price - ws_ext_discount_amt) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE 0.0 END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE 0.0 END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country LIMIT 100",
			},
		},
		{
			name: "tpcds5",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH ss AS (SELECT ca_county, d_qoy, d_year, Sum(ss_ext_sales_price) AS store_sales FROM store_sales, date_dim, customer_address WHERE ss_sold_date_sk = d_date_sk AND ss_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year), ws AS (SELECT ca_county, d_qoy, d_year, Sum(ws_ext_sales_price) AS web_sales FROM web_sales, date_dim, customer_address WHERE ws_sold_date_sk = d_date_sk AND ws_bill_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year) SELECT ss1.ca_county, ss1.d_year, ws2.web_sales / ws1.web_sales web_q1_q2_increase, ss2.store_sales / ss1.store_sales store_q1_q2_increase, ws3.web_sales / ws2.web_sales web_q2_q3_increase, ss3.store_sales / ss2.store_sales store_q2_q3_increase FROM ss ss1, ss ss2, ss ss3, ws ws1, ws ws2, ws ws3 WHERE ss1.d_qoy = 1 AND ss1.d_year = 2001 AND ss1.ca_county = ss2.ca_county AND ss2.d_qoy = 2 AND ss2.d_year = 2001 AND ss2.ca_county = ss3.ca_county AND ss3.d_qoy = 3 AND ss3.d_year = 2001 AND ss1.ca_county = ws1.ca_county AND ws1.d_qoy = 1 AND ws1.d_year = 2001 AND ws1.ca_county = ws2.ca_county AND ws2.d_qoy = 2 AND ws2.d_year = 2001 AND ws1.ca_county = ws3.ca_county AND ws3.d_qoy = 3 AND ws3.d_year = 2001 AND CASE WHEN ws1.web_sales > 0 THEN ws2.web_sales / ws1.web_sales ELSE NULL END > CASE WHEN ss1.store_sales > 0 THEN ss2.store_sales / ss1.store_sales ELSE NULL END AND CASE WHEN ws2.web_sales > 0 THEN ws3.web_sales / ws2.web_sales ELSE NULL END > CASE WHEN ss2.store_sales > 0 THEN ss3.store_sales / ss2.store_sales ELSE NULL END ORDER BY ss1.d_year",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH ss AS (SELECT ca_county, d_qoy, d_year, Sum(ss_ext_sales_price) AS store_sales FROM store_sales, date_dim, customer_address WHERE ss_sold_date_sk = d_date_sk AND ss_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year), ws AS (SELECT ca_county, d_qoy, d_year, Sum(ws_ext_sales_price) AS web_sales FROM web_sales, date_dim, customer_address WHERE ws_sold_date_sk = d_date_sk AND ws_bill_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year) SELECT ss1.ca_county, ss1.d_year, ws2.web_sales / ws1.web_sales web_q1_q2_increase, ss2.store_sales / ss1.store_sales store_q1_q2_increase, ws3.web_sales / ws2.web_sales web_q2_q3_increase, ss3.store_sales / ss2.store_sales store_q2_q3_increase FROM ss ss1, ss ss2, ss ss3, ws ws1, ws ws2, ws ws3 WHERE ss1.d_qoy = 1 AND ss1.d_year = 2001 AND ss1.ca_county = ss2.ca_county AND ss2.d_qoy = 2 AND ss2.d_year = 2001 AND ss2.ca_county = ss3.ca_county AND ss3.d_qoy = 3 AND ss3.d_year = 2001 AND ss1.ca_county = ws1.ca_county AND ws1.d_qoy = 1 AND ws1.d_year = 2001 AND ws1.ca_county = ws2.ca_county AND ws2.d_qoy = 2 AND ws2.d_year = 2001 AND ws1.ca_county = ws3.ca_county AND ws3.d_qoy = 3 AND ws3.d_year = 2001 AND CASE WHEN ws1.web_sales > 0 THEN ws2.web_sales / ws1.web_sales ELSE NULL END > CASE WHEN ss1.store_sales > 0 THEN ss2.store_sales / ss1.store_sales ELSE NULL END AND CASE WHEN ws2.web_sales > 0 THEN ws3.web_sales / ws2.web_sales ELSE NULL END > CASE WHEN ss2.store_sales > 0 THEN ss3.store_sales / ss2.store_sales ELSE NULL END ORDER BY ss1.d_year",
			},
		},
		{
			name: "tpcds6",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH wss AS (SELECT d_week_seq, ss_store_sk, Sum(CASE WHEN ( d_day_name = 'Sunday' ) THEN ss_sales_price ELSE NULL END) sun_sales, Sum(CASE WHEN ( d_day_name = 'Monday' ) THEN ss_sales_price ELSE NULL END) mon_sales, Sum(CASE WHEN ( d_day_name = 'Tuesday' ) THEN ss_sales_price ELSE NULL END) tue_sales, Sum(CASE WHEN ( d_day_name = 'Wednesday' ) THEN ss_sales_price ELSE NULL END) wed_sales, Sum(CASE WHEN ( d_day_name = 'Thursday' ) THEN ss_sales_price ELSE NULL END) thu_sales, Sum(CASE WHEN ( d_day_name = 'Friday' ) THEN ss_sales_price ELSE NULL END) fri_sales, Sum(CASE WHEN ( d_day_name = 'Saturday' ) THEN ss_sales_price ELSE NULL END) sat_sales FROM store_sales, date_dim WHERE d_date_sk = ss_sold_date_sk GROUP BY d_week_seq, ss_store_sk) SELECT s_store_name1, s_store_id1, d_week_seq1, sun_sales1 / sun_sales2, mon_sales1 / mon_sales2, tue_sales1 / tue_sales2, wed_sales1 / wed_sales2, thu_sales1 / thu_sales2, fri_sales1 / fri_sales2, sat_sales1 / sat_sales2 FROM (SELECT s_store_name s_store_name1, wss.d_week_seq d_week_seq1, s_store_id s_store_id1, sun_sales sun_sales1, mon_sales mon_sales1, tue_sales tue_sales1, wed_sales wed_sales1, thu_sales thu_sales1, fri_sales fri_sales1, sat_sales sat_sales1 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 AND 1196 + 11) y, (SELECT s_store_name s_store_name2, wss.d_week_seq d_week_seq2, s_store_id s_store_id2, sun_sales sun_sales2, mon_sales mon_sales2, tue_sales tue_sales2, wed_sales wed_sales2, thu_sales thu_sales2, fri_sales fri_sales2, sat_sales sat_sales2 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 + 12 AND 1196 + 23) x WHERE s_store_id1 = s_store_id2 AND d_week_seq1 = d_week_seq2 - 52 ORDER BY s_store_name1, s_store_id1, d_week_seq1 LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH wss AS (SELECT d_week_seq, ss_store_sk, Sum(CASE WHEN ( d_day_name = 'Sunday' ) THEN ss_sales_price ELSE NULL END) sun_sales, Sum(CASE WHEN ( d_day_name = 'Monday' ) THEN ss_sales_price ELSE NULL END) mon_sales, Sum(CASE WHEN ( d_day_name = 'Tuesday' ) THEN ss_sales_price ELSE NULL END) tue_sales, Sum(CASE WHEN ( d_day_name = 'Wednesday' ) THEN ss_sales_price ELSE NULL END) wed_sales, Sum(CASE WHEN ( d_day_name = 'Thursday' ) THEN ss_sales_price ELSE NULL END) thu_sales, Sum(CASE WHEN ( d_day_name = 'Friday' ) THEN ss_sales_price ELSE NULL END) fri_sales, Sum(CASE WHEN ( d_day_name = 'Saturday' ) THEN ss_sales_price ELSE NULL END) sat_sales FROM store_sales, date_dim WHERE d_date_sk = ss_sold_date_sk GROUP BY d_week_seq, ss_store_sk) SELECT s_store_name1, s_store_id1, d_week_seq1, sun_sales1 / sun_sales2, mon_sales1 / mon_sales2, tue_sales1 / tue_sales2, wed_sales1 / wed_sales2, thu_sales1 / thu_sales2, fri_sales1 / fri_sales2, sat_sales1 / sat_sales2 FROM (SELECT s_store_name s_store_name1, wss.d_week_seq d_week_seq1, s_store_id s_store_id1, sun_sales sun_sales1, mon_sales mon_sales1, tue_sales tue_sales1, wed_sales wed_sales1, thu_sales thu_sales1, fri_sales fri_sales1, sat_sales sat_sales1 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 AND 1196 + 11) y, (SELECT s_store_name s_store_name2, wss.d_week_seq d_week_seq2, s_store_id s_store_id2, sun_sales sun_sales2, mon_sales mon_sales2, tue_sales tue_sales2, wed_sales wed_sales2, thu_sales thu_sales2, fri_sales fri_sales2, sat_sales sat_sales2 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 + 12 AND 1196 + 23) x WHERE s_store_id1 = s_store_id2 AND d_week_seq1 = d_week_seq2 - 52 ORDER BY s_store_name1, s_store_id1, d_week_seq1 LIMIT 100",
			},
		},
		{
			name: "tpcds7",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH all_sales AS (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, Sum(sales_cnt) AS sales_cnt, Sum(sales_amt) AS sales_amt FROM (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, cs_quantity - COALESCE(cr_return_quantity, 0) AS sales_cnt, cs_ext_sales_price - COALESCE(cr_return_amount, 0.0) AS sales_amt FROM catalog_sales JOIN item ON i_item_sk = cs_item_sk JOIN date_dim ON d_date_sk = cs_sold_date_sk LEFT JOIN catalog_returns ON ( cs_order_number = cr_order_number AND cs_item_sk = cr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ss_quantity - COALESCE(sr_return_quantity, 0) AS sales_cnt, ss_ext_sales_price - COALESCE(sr_return_amt, 0.0) AS sales_amt FROM store_sales JOIN item ON i_item_sk = ss_item_sk JOIN date_dim ON d_date_sk = ss_sold_date_sk LEFT JOIN store_returns ON ( ss_ticket_number = sr_ticket_number AND ss_item_sk = sr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ws_quantity - COALESCE(wr_return_quantity, 0) AS sales_cnt, ws_ext_sales_price - COALESCE(wr_return_amt, 0.0) AS sales_amt FROM web_sales JOIN item ON i_item_sk = ws_item_sk JOIN date_dim ON d_date_sk = ws_sold_date_sk LEFT JOIN web_returns ON ( ws_order_number = wr_order_number AND ws_item_sk = wr_item_sk ) WHERE i_category = 'Men') sales_detail GROUP BY d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id) SELECT prev_yr.d_year AS prev_year, curr_yr.d_year AS year1, curr_yr.i_brand_id, curr_yr.i_class_id, curr_yr.i_category_id, curr_yr.i_manufact_id, prev_yr.sales_cnt AS prev_yr_cnt, curr_yr.sales_cnt AS curr_yr_cnt, curr_yr.sales_cnt - prev_yr.sales_cnt AS sales_cnt_diff, curr_yr.sales_amt - prev_yr.sales_amt AS sales_amt_diff FROM all_sales curr_yr, all_sales prev_yr WHERE curr_yr.i_brand_id = prev_yr.i_brand_id AND curr_yr.i_class_id = prev_yr.i_class_id AND curr_yr.i_category_id = prev_yr.i_category_id AND curr_yr.i_manufact_id = prev_yr.i_manufact_id AND curr_yr.d_year = 2002 AND prev_yr.d_year = 2002 - 1 AND Cast(curr_yr.sales_cnt AS DECIMAL(17, 2)) / Cast(prev_yr.sales_cnt AS DECIMAL(17, 2)) < 0.9 ORDER BY sales_cnt_diff LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH all_sales AS (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, Sum(sales_cnt) AS sales_cnt, Sum(sales_amt) AS sales_amt FROM (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, cs_quantity - COALESCE(cr_return_quantity, 0) AS sales_cnt, cs_ext_sales_price - COALESCE(cr_return_amount, 0.0) AS sales_amt FROM catalog_sales JOIN item ON i_item_sk = cs_item_sk JOIN date_dim ON d_date_sk = cs_sold_date_sk LEFT JOIN catalog_returns ON ( cs_order_number = cr_order_number AND cs_item_sk = cr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ss_quantity - COALESCE(sr_return_quantity, 0) AS sales_cnt, ss_ext_sales_price - COALESCE(sr_return_amt, 0.0) AS sales_amt FROM store_sales JOIN item ON i_item_sk = ss_item_sk JOIN date_dim ON d_date_sk = ss_sold_date_sk LEFT JOIN store_returns ON ( ss_ticket_number = sr_ticket_number AND ss_item_sk = sr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ws_quantity - COALESCE(wr_return_quantity, 0) AS sales_cnt, ws_ext_sales_price - COALESCE(wr_return_amt, 0.0) AS sales_amt FROM web_sales JOIN item ON i_item_sk = ws_item_sk JOIN date_dim ON d_date_sk = ws_sold_date_sk LEFT JOIN web_returns ON ( ws_order_number = wr_order_number AND ws_item_sk = wr_item_sk ) WHERE i_category = 'Men') sales_detail GROUP BY d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id) SELECT prev_yr.d_year AS prev_year, curr_yr.d_year AS year1, curr_yr.i_brand_id, curr_yr.i_class_id, curr_yr.i_category_id, curr_yr.i_manufact_id, prev_yr.sales_cnt AS prev_yr_cnt, curr_yr.sales_cnt AS curr_yr_cnt, curr_yr.sales_cnt - prev_yr.sales_cnt AS sales_cnt_diff, curr_yr.sales_amt - prev_yr.sales_amt AS sales_amt_diff FROM all_sales curr_yr, all_sales prev_yr WHERE curr_yr.i_brand_id = prev_yr.i_brand_id AND curr_yr.i_class_id = prev_yr.i_class_id AND curr_yr.i_category_id = prev_yr.i_category_id AND curr_yr.i_manufact_id = prev_yr.i_manufact_id AND curr_yr.d_year = 2002 AND prev_yr.d_year = 2002 - 1 AND Cast(curr_yr.sales_cnt AS DECIMAL(17, 2)) / Cast(prev_yr.sales_cnt AS DECIMAL(17, 2)) < 0.9 ORDER BY sales_cnt_diff LIMIT 100",
			},
		},
		{
			name: "tpcds8",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH customer_total_return AS (SELECT cr_returning_customer_sk AS ctr_customer_sk, ca_state AS ctr_state, Sum(cr_return_amt_inc_tax) AS ctr_total_return FROM catalog_returns, date_dim, customer_address WHERE cr_returned_date_sk = d_date_sk AND d_year = 1999 AND cr_returning_addr_sk = ca_address_sk GROUP BY cr_returning_customer_sk, ca_state) SELECT c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return FROM customer_total_return ctr1, customer_address, customer WHERE ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_state = ctr2.ctr_state) AND ca_address_sk = c_current_addr_sk AND ca_state = 'TX' AND ctr1.ctr_customer_sk = c_customer_sk ORDER BY c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return LIMIT 100; ## Q95 WITH ws_wh AS ( SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH customer_total_return AS (SELECT cr_returning_customer_sk AS ctr_customer_sk, ca_state AS ctr_state, Sum(cr_return_amt_inc_tax) AS ctr_total_return FROM catalog_returns, date_dim, customer_address WHERE cr_returned_date_sk = d_date_sk AND d_year = 1999 AND cr_returning_addr_sk = ca_address_sk GROUP BY cr_returning_customer_sk, ca_state) SELECT c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return FROM customer_total_return ctr1, customer_address, customer WHERE ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_state = ctr2.ctr_state) AND ca_address_sk = c_current_addr_sk AND ca_state = 'TX' AND ctr1.ctr_customer_sk = c_customer_sk ORDER BY c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return LIMIT 100; ## Q95 WITH ws_wh AS ( SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100",
			},
		},
		{
			name: "tpcds9",
			expected: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					"set tidb_max_bytes_before_tiflash_cte_spill=200000000000000",
				},
				query: "WITH ws_wh AS (SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100",
			},
			actual: compareSQL{
				setupSQLs: []string{
					"set tidb_enforce_mpp=1",
					"set tidb_opt_enable_mpp_shared_cte_execution=on",
					fmt.Sprintf("set tidb_max_bytes_before_tiflash_cte_spill=%d", rand.Intn(20000000)+10),
				},
				query: "WITH ws_wh AS (SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100",
			},
		},
		// --------------------- Customized Dataset ---------------------
	}
)

// ------------------------------------------------------------

type compareSQL struct {
	setupSQLs []string
	query     string
}

type compareSQLPair struct {
	name     string
	expected compareSQL
	actual   compareSQL
}

type compareCell struct {
	value  string
	isNull bool
}

type compareRow []compareCell

const compareSQLLogMaxCharacters = 100

type compareStageTimings struct {
	totalStart         time.Time
	expectedDuration   time.Duration
	actualDuration     time.Duration
	comparisonDuration time.Duration
}

var compareLogMu sync.Mutex

func writeCompareLog(format string, args ...any) {
	compareLogMu.Lock()
	defer compareLogMu.Unlock()
	fmt.Printf(format+"\n", args...)
}

func (timings compareStageTimings) printSummary(label string, resultErr error) {
	status := "success"
	if resultErr != nil {
		status = "failure"
	}
	writeCompareLog(
		"[%s] Finished: status=%s, total=%s, expected=%s, actual=%s, comparison=%s",
		label,
		status,
		time.Since(timings.totalStart),
		timings.expectedDuration,
		timings.actualDuration,
		timings.comparisonDuration,
	)
}

// runCompareResults packages the complete comparison as one task and hands it
// to runner. runner itself is deliberately single-threaded; optional SQL-pair
// concurrency is implemented inside the task.
func runCompareResults() error {
	var compareErr error
	task := func() {
		compareErr = compareResultsTask(context.Background())
	}
	newRunner([]func(){task}).run()
	return compareErr
}

func compareResultsTask(ctx context.Context) error {
	printCompareResultConfig()
	if compareResultReadFromFiles {
		return compareConfiguredFiles()
	}
	return compareConfiguredSQLPairs(ctx)
}

func printCompareResultConfig() {
	dataSource := "sql"
	if compareResultReadFromFiles {
		dataSource = "files"
	}

	fmt.Println("========== COMPARE CONFIG ==========")
	fmt.Printf("compareResultSortRows=%t\n", compareResultSortRows)
	fmt.Printf("compareResultCaseSensitive=%t\n", compareResultCaseSensitive)
	fmt.Printf("compareResultRunConcurrently=%t\n", compareResultRunConcurrently)
	fmt.Printf("compareResultConcurrentWorkerCount=%d\n", compareResultConcurrentWorkerCount)
	fmt.Printf("compareResultConcurrentRunDuration=%s\n", compareResultConcurrentRunDuration)
	fmt.Printf("compareResultConcurrentRunsPerPair=%d\n", compareResultConcurrentRunsPerPair)
	fmt.Printf("compareResultReadFromFiles=%t\n", compareResultReadFromFiles)
	fmt.Printf("compareResultDataSource=%s\n", dataSource)
	fmt.Printf("compareResultExpectedFile=%q\n", compareResultExpectedFile)
	fmt.Printf("compareResultActualFile=%q\n", compareResultActualFile)
	fmt.Printf("compareResultDBConfig.address=%q\n", compareResultDBConfig.address)
	fmt.Printf("compareResultDBConfig.port=%q\n", compareResultDBConfig.port)
	fmt.Printf("compareResultDBConfig.user=%q\n", compareResultDBConfig.user)
	fmt.Printf("compareResultDBConfig.dbName=%q\n", compareResultDBConfig.dbName)
	fmt.Printf("compareResultDBConfig.params=%q\n", compareResultDBConfig.params)
	fmt.Printf("compareResultSQLPairs.count=%d\n", len(compareResultSQLPairs))
	// for pairIndex, pair := range compareResultSQLPairs {
	// 	pairPrefix := fmt.Sprintf("compareResultSQLPairs[%d]", pairIndex)
	// 	fmt.Printf("%s.name=%q\n", pairPrefix, pair.name)
	// 	printCompareSQLConfig(pairPrefix+".expected", pair.expected)
	// 	printCompareSQLConfig(pairPrefix+".actual", pair.actual)
	// }
	fmt.Println("========== END COMPARE CONFIG ==========")
}

func printCompareSQLConfig(prefix string, configuredSQL compareSQL) {
	fmt.Printf("%s.setupSQLs.count=%d\n", prefix, len(configuredSQL.setupSQLs))
	for setupIndex, setupSQL := range configuredSQL.setupSQLs {
		fmt.Printf("%s.setupSQLs[%d]=%q\n", prefix, setupIndex, setupSQL)
	}
	fmt.Printf("%s.query=%q\n", prefix, configuredSQL.query)
}

func compareConfiguredFiles() (resultErr error) {
	label := fmt.Sprintf("File comparison %q vs %q", compareResultExpectedFile, compareResultActualFile)
	timings := compareStageTimings{totalStart: time.Now()}
	defer func() {
		timings.printSummary(label, resultErr)
	}()

	if strings.TrimSpace(compareResultExpectedFile) == "" || strings.TrimSpace(compareResultActualFile) == "" {
		return errors.New("file comparison requires both compareResultExpectedFile and compareResultActualFile")
	}

	expectedStart := time.Now()
	writeCompareLog(
		"[%s] Reading expected result file, start_time=%s",
		label,
		expectedStart.Format(time.RFC3339Nano),
	)
	expected, err := readResultFile(compareResultExpectedFile)
	timings.expectedDuration = time.Since(expectedStart)
	if err != nil {
		return fmt.Errorf("read expected result file %q: %w", compareResultExpectedFile, err)
	}
	actualStart := time.Now()
	writeCompareLog(
		"[%s] Reading actual result file, start_time=%s",
		label,
		actualStart.Format(time.RFC3339Nano),
	)
	actual, err := readResultFile(compareResultActualFile)
	timings.actualDuration = time.Since(actualStart)
	if err != nil {
		return fmt.Errorf("read actual result file %q: %w", compareResultActualFile, err)
	}
	comparisonStart := time.Now()
	writeCompareLog(
		"[%s] Comparing expected and actual result sets, start_time=%s",
		label,
		comparisonStart.Format(time.RFC3339Nano),
	)
	if err := compareResultRows(expected, actual); err != nil {
		timings.comparisonDuration = time.Since(comparisonStart)
		return err
	}
	timings.comparisonDuration = time.Since(comparisonStart)
	writeCompareLog("[%s] Success", label)
	return nil
}

func compareConfiguredSQLPairs(ctx context.Context) error {
	if len(compareResultSQLPairs) == 0 {
		return errors.New("SQL comparison requires at least one entry in compareResultSQLPairs")
	}
	if compareResultRunConcurrently {
		if err := validateConcurrentCompareConfig(); err != nil {
			return err
		}
	}

	db, err := getDB(compareResultDBConfig)
	if err != nil {
		return fmt.Errorf("connect to comparison database: %w", err)
	}
	defer db.Close()

	// Session variables set by setupSQLs must not leak into another SQL in a
	// later case. With no idle connections, releasing *sql.Conn also closes the
	// underlying connection and therefore its session state.
	db.SetMaxIdleConns(0)

	if compareResultRunConcurrently {
		return compareSQLPairsConcurrently(ctx, db, compareResultSQLPairs)
	}
	return compareSQLPairsSerially(ctx, db, compareResultSQLPairs)
}

func compareSQLPairsSerially(ctx context.Context, db *sql.DB, pairs []compareSQLPair) error {
	for i, pair := range pairs {
		label := sqlPairLabel(i, pair)
		writeCompareLog("[%s] Starting comparison", label)
		if err := compareOneSQLPairWithLabel(ctx, db, pair, label); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		writeCompareLog("[%s] Success", label)
	}
	return nil
}

func compareSQLPairsConcurrently(ctx context.Context, db *sql.DB, pairs []compareSQLPair) error {
	return runSQLPairsRandomly(ctx, pairs, func(ctx context.Context, pairIndex int, pair compareSQLPair) error {
		return compareOneSQLPairWithLabel(ctx, db, pair, sqlPairLabel(pairIndex, pair))
	})
}

type comparePairFunc func(context.Context, int, compareSQLPair) error

// randomPairScheduler randomly chooses one currently available pair. In
// per-pair-count mode, a pair is removed from the available range as soon as
// its configured number of runs has been reserved.
type randomPairScheduler struct {
	mu               sync.Mutex
	random           *rand.Rand
	runsPerPairLimit int
	runCounts        []int
	availablePairs   []int
}

func newRandomPairScheduler(pairCount, runsPerPairLimit int) *randomPairScheduler {
	availablePairs := make([]int, pairCount)
	for i := range availablePairs {
		availablePairs[i] = i
	}
	return &randomPairScheduler{
		random:           rand.New(rand.NewSource(time.Now().UnixNano())),
		runsPerPairLimit: runsPerPairLimit,
		runCounts:        make([]int, pairCount),
		availablePairs:   availablePairs,
	}
}

func (s *randomPairScheduler) next() (pairIndex, runNumber int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.availablePairs) == 0 {
		return 0, 0, false
	}
	availableIndex := s.random.Intn(len(s.availablePairs))
	pairIndex = s.availablePairs[availableIndex]
	s.runCounts[pairIndex]++
	runNumber = s.runCounts[pairIndex]

	if s.runsPerPairLimit > 0 && runNumber >= s.runsPerPairLimit {
		last := len(s.availablePairs) - 1
		s.availablePairs[availableIndex] = s.availablePairs[last]
		s.availablePairs = s.availablePairs[:last]
	}
	return pairIndex, runNumber, true
}

func (s *randomPairScheduler) scheduledRunCounts() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.runCounts...)
}

func validateConcurrentCompareConfig() error {
	if compareResultConcurrentWorkerCount <= 0 {
		return errors.New("compareResultConcurrentWorkerCount must be greater than zero")
	}
	if compareResultConcurrentRunDuration < 0 {
		return errors.New("compareResultConcurrentRunDuration cannot be negative")
	}
	if compareResultConcurrentRunsPerPair < 0 {
		return errors.New("compareResultConcurrentRunsPerPair cannot be negative")
	}
	if compareResultConcurrentRunDuration == 0 && compareResultConcurrentRunsPerPair == 0 {
		return errors.New("concurrent comparison requires a run-duration or runs-per-pair limit")
	}
	return nil
}

func runSQLPairsRandomly(ctx context.Context, pairs []compareSQLPair, comparePair comparePairFunc) error {
	if len(pairs) == 0 {
		return errors.New("random SQL comparison requires at least one SQL pair")
	}
	if comparePair == nil {
		return errors.New("random SQL comparison requires a comparison function")
	}
	if err := validateConcurrentCompareConfig(); err != nil {
		return err
	}

	workerCount := compareResultConcurrentWorkerCount
	runDuration := compareResultConcurrentRunDuration
	runsPerPair := compareResultConcurrentRunsPerPair

	limitDescription := make([]string, 0, 2)
	if runDuration > 0 {
		limitDescription = append(limitDescription, fmt.Sprintf("duration=%s", runDuration))
	}
	if runsPerPair > 0 {
		limitDescription = append(limitDescription, fmt.Sprintf("runs-per-pair=%d", runsPerPair))
	}
	fmt.Printf(
		"Start concurrent random comparison: workers=%d, SQL pairs=%d, %s\n",
		workerCount,
		len(pairs),
		strings.Join(limitDescription, ", "),
	)

	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	schedulingCtx := workerCtx
	if runDuration > 0 {
		var stopTimer context.CancelFunc
		schedulingCtx, stopTimer = context.WithTimeout(workerCtx, runDuration)
		defer stopTimer()
	}

	scheduler := newRandomPairScheduler(len(pairs), runsPerPair)
	completedRuns := make([]atomic.Int64, len(pairs))
	var firstFailure error
	var failureOnce sync.Once
	var outputMu sync.Mutex
	writeRunLog := func(format string, args ...any) {
		outputMu.Lock()
		defer outputMu.Unlock()
		fmt.Printf(format+"\n", args...)
	}
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for workerIndex := range workerCount {
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-schedulingCtx.Done():
					return
				default:
				}

				pairIndex, runNumber, ok := scheduler.next()
				if !ok {
					return
				}

				label := sqlPairLabel(pairIndex, pairs[pairIndex])
				startedAt := time.Now()
				writeRunLog("[worker %d] Start %s, run %d", workerID, label, runNumber)
				if err := comparePair(workerCtx, pairIndex, pairs[pairIndex]); err != nil {
					elapsed := time.Since(startedAt)
					// Other in-flight queries may be interrupted after one worker has
					// already reported the real failure. Do not replace that first error
					// with a secondary context-cancellation error.
					if workerCtx.Err() != nil {
						writeRunLog(
							"[worker %d] Stopped %s, run %d, elapsed=%s, error=%v",
							workerID,
							label,
							runNumber,
							elapsed,
							err,
						)
						return
					}
					writeRunLog(
						"[worker %d] Failed %s, run %d, elapsed=%s, error=%v",
						workerID,
						label,
						runNumber,
						elapsed,
						err,
					)
					failureOnce.Do(func() {
						firstFailure = fmt.Errorf(
							"%s, run %d: %w",
							label,
							runNumber,
							err,
						)
						stopWorkers()
					})
					return
				}
				completed := completedRuns[pairIndex].Add(1)
				writeRunLog(
					"[worker %d] Success %s, run %d, elapsed=%s, successful=%d",
					workerID,
					label,
					runNumber,
					time.Since(startedAt),
					completed,
				)
			}
		}(workerIndex + 1)
	}
	wg.Wait()

	scheduledRuns := scheduler.scheduledRunCounts()
	for i := range pairs {
		fmt.Printf(
			"Completed %s: successful=%d, scheduled=%d\n",
			sqlPairLabel(i, pairs[i]),
			completedRuns[i].Load(),
			scheduledRuns[i],
		)
	}

	if firstFailure != nil {
		return firstFailure
	}
	if ctx.Err() != nil {
		return fmt.Errorf("concurrent comparison canceled: %w", ctx.Err())
	}
	fmt.Println("Success: concurrent random comparison finished")
	return nil
}

func sqlPairLabel(index int, pair compareSQLPair) string {
	if strings.TrimSpace(pair.name) == "" {
		return fmt.Sprintf("SQL pair %d", index+1)
	}
	return fmt.Sprintf("SQL pair %d (%s)", index+1, pair.name)
}

func compareOneSQLPair(ctx context.Context, db *sql.DB, pair compareSQLPair) error {
	label := "SQL pair"
	if strings.TrimSpace(pair.name) != "" {
		label = fmt.Sprintf("SQL pair (%s)", pair.name)
	}
	return compareOneSQLPairWithLabel(ctx, db, pair, label)
}

func compareOneSQLPairWithLabel(ctx context.Context, db *sql.DB, pair compareSQLPair, label string) (resultErr error) {
	timings := compareStageTimings{totalStart: time.Now()}
	defer func() {
		timings.printSummary(label, resultErr)
	}()

	expectedStart := time.Now()
	writeCompareLog(
		"[%s] Executing expected SQL: %q, start_time=%s",
		label,
		summarizeCompareSQLForLog(pair.expected.query),
		expectedStart.Format(time.RFC3339Nano),
	)
	expected, err := executeCompareSQL(ctx, db, pair.expected)
	timings.expectedDuration = time.Since(expectedStart)
	if err != nil {
		return fmt.Errorf("execute expected SQL: %w", err)
	}

	actualStart := time.Now()
	writeCompareLog(
		"[%s] Executing actual SQL: %q, start_time=%s",
		label,
		summarizeCompareSQLForLog(pair.actual.query),
		actualStart.Format(time.RFC3339Nano),
	)
	actual, err := executeCompareSQL(ctx, db, pair.actual)
	timings.actualDuration = time.Since(actualStart)
	if err != nil {
		return fmt.Errorf("execute actual SQL: %w", err)
	}

	comparisonStart := time.Now()
	writeCompareLog(
		"[%s] Comparing expected and actual result sets, start_time=%s",
		label,
		comparisonStart.Format(time.RFC3339Nano),
	)
	resultErr = compareResultRows(expected, actual)
	timings.comparisonDuration = time.Since(comparisonStart)
	return resultErr
}

func summarizeCompareSQLForLog(query string) string {
	query = strings.Join(strings.Fields(query), " ")
	if utf8.RuneCountInString(query) <= compareSQLLogMaxCharacters {
		return query
	}
	runes := []rune(query)
	return string(runes[:compareSQLLogMaxCharacters-3]) + "..."
}

// executeCompareSQL keeps setupSQLs and query on the same connection. This is
// essential for session-scoped statements such as SET and USE.
func executeCompareSQL(ctx context.Context, db *sql.DB, configuredSQL compareSQL) ([]compareRow, error) {
	if strings.TrimSpace(configuredSQL.query) == "" {
		return nil, errors.New("query cannot be empty")
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Close()

	for i, setupSQL := range configuredSQL.setupSQLs {
		if strings.TrimSpace(setupSQL) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, setupSQL); err != nil {
			return nil, fmt.Errorf("execute setup SQL %d (%q): %w", i+1, setupSQL, err)
		}
	}

	rows, err := conn.QueryContext(ctx, configuredSQL.query)
	if err != nil {
		return nil, fmt.Errorf("execute query %q: %w", configuredSQL.query, err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read result columns: %w", err)
	}
	result := make([]compareRow, 0)
	for rows.Next() {
		values := make([]any, len(columnNames))
		destinations := make([]any, len(columnNames))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, fmt.Errorf("scan result row %d: %w", len(result), err)
		}

		row := make(compareRow, len(values))
		for i, value := range values {
			row[i] = databaseValueToCompareCell(value)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate result rows: %w", err)
	}
	return result, nil
}

func databaseValueToCompareCell(value any) compareCell {
	switch value := value.(type) {
	case nil:
		return compareCell{isNull: true}
	case []byte:
		return compareCell{value: string(value)}
	case string:
		return compareCell{value: value}
	default:
		return compareCell{value: fmt.Sprint(value)}
	}
}

// readResultFile has the same row boundaries as Python's text-mode readlines:
// line endings are retained and CRLF/CR are normalized to LF.
func readResultFile(fileName string) ([]compareRow, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, errors.New("result file is not valid UTF-8")
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return []compareRow{}, nil
	}

	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	rows := make([]compareRow, len(lines))
	for i, line := range lines {
		rows[i] = compareRow{{value: line}}
	}
	return rows, nil
}

func compareResultRows(expected, actual []compareRow) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length not equal, %d vs %d", len(expected), len(actual))
	}

	if compareResultSortRows {
		expected = sortedResultRows(expected)
		actual = sortedResultRows(actual)
	}

	for i := range expected {
		if !compareRowsEqual(expected[i], actual[i]) {
			return fmt.Errorf(
				"Incorrect answer: row %d, <%s> vs <%s>",
				i,
				formatCompareRow(expected[i]),
				formatCompareRow(actual[i]),
			)
		}
	}
	return nil
}

func sortedResultRows(rows []compareRow) []compareRow {
	result := append([]compareRow(nil), rows...)
	sort.SliceStable(result, func(i, j int) bool {
		return compareRowKey(result[i]) < compareRowKey(result[j])
	})
	return result
}

func compareRowsEqual(expected, actual compareRow) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].isNull != actual[i].isNull {
			return false
		}
		if expected[i].isNull {
			continue
		}
		if normalizeCompareText(expected[i].value) != normalizeCompareText(actual[i].value) {
			return false
		}
	}
	return true
}

func compareRowKey(row compareRow) string {
	var key strings.Builder
	for _, cell := range row {
		if cell.isNull {
			key.WriteString("N;")
			continue
		}
		value := normalizeCompareText(cell.value)
		key.WriteByte('V')
		key.WriteString(strconv.Itoa(len(value)))
		key.WriteByte(':')
		key.WriteString(value)
		key.WriteByte(';')
	}
	return key.String()
}

func normalizeCompareText(value string) string {
	if compareResultCaseSensitive {
		return value
	}

	// Canonicalize every Unicode simple-fold cycle to its smallest rune. This
	// gives both equality checks and sorting one stable key, including cases such
	// as Greek sigma (Σ/σ/ς) that strings.ToLower alone does not fully cover.
	var normalized strings.Builder
	for _, current := range value {
		canonical := current
		for folded := unicode.SimpleFold(current); folded != current; folded = unicode.SimpleFold(folded) {
			if folded < canonical {
				canonical = folded
			}
		}
		normalized.WriteRune(canonical)
	}
	return normalized.String()
}

func formatCompareRow(row compareRow) string {
	values := make([]string, len(row))
	for i, cell := range row {
		if cell.isNull {
			values[i] = "NULL"
			continue
		}
		values[i] = cell.value
	}
	return strings.Join(values, "\t")
}
