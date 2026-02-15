import pymysql
import threading
import time
import random
from datetime import datetime
import re

host = "10.2.12.124"
port = 8001
database = "test"

expect_results = []

sqls = []

tpcds_spill_sqls = [
    # 200w
    "WITH customer_total_return AS (SELECT sr_customer_sk AS ctr_customer_sk, sr_store_sk AS ctr_store_sk, Sum(sr_return_amt) AS ctr_total_return FROM store_returns, date_dim WHERE sr_returned_date_sk = d_date_sk AND d_year = 2001 GROUP BY sr_customer_sk, sr_store_sk) SELECT c_customer_id FROM customer_total_return ctr1, store, customer WHERE  ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_store_sk = ctr2.ctr_store_sk) AND s_store_sk = ctr1.ctr_store_sk AND s_state = 'TN' AND ctr1.ctr_customer_sk = c_customer_sk ORDER  BY c_customer_id LIMIT 100;",
    # 180w
    "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(( ( ss_ext_list_price - ss_ext_wholesale_cost - ss_ext_discount_amt ) + ss_ext_sales_price ) / 2) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( cs_ext_list_price - cs_ext_wholesale_cost - cs_ext_discount_amt ) + cs_ext_sales_price ) / 2 )) year_total FROM customer, catalog_sales, date_dim WHERE c_customer_sk = cs_bill_customer_sk AND cs_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( ws_ext_list_price - ws_ext_wholesale_cost - ws_ext_discount_amt ) + ws_ext_sales_price ) / 2 )) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_c_firstyear, year_total t_c_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_c_secyear.customer_id AND t_s_firstyear.customer_id = t_c_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_c_firstyear.dyear = 2001 AND t_c_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_c_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE NULL END AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE NULL END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag LIMIT 100;",
    # 100w
    "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ss_ext_list_price - ss_ext_discount_amt) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ws_ext_list_price - ws_ext_discount_amt) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE 0.0 END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE 0.0 END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country LIMIT 100;",
    # 130w
    "WITH customer_total_return AS (SELECT cr_returning_customer_sk AS ctr_customer_sk, ca_state AS ctr_state, Sum(cr_return_amt_inc_tax) AS ctr_total_return FROM catalog_returns, date_dim, customer_address WHERE cr_returned_date_sk = d_date_sk AND d_year = 1999 AND cr_returning_addr_sk = ca_address_sk GROUP BY cr_returning_customer_sk, ca_state) SELECT c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return FROM customer_total_return ctr1, customer_address, customer WHERE ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_state = ctr2.ctr_state) AND ca_address_sk = c_current_addr_sk AND ca_state = 'TX' AND ctr1.ctr_customer_sk = c_customer_sk ORDER BY c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return LIMIT 100; ## Q95 WITH ws_wh AS ( SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100;",
    # 30000w
    "WITH ws_wh AS (SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100;",
]

tpch_spill_sqls = [
    # 200w
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 18, 10) as col1 from part) select t1.p_partkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    # 800w
    "with cte1 as (select ps_partkey, ps_suppkey, (ps_supplycost + ps_partkey) * 13 as col0, (ps_supplycost + ps_suppkey) * 13 as col1 from partsupp) select t1.ps_partkey, t1.ps_suppkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    # 1500w
    "with cte1 as (select o_orderkey, date_add(o_orderdate, interval o_orderkey%10000000 hour) as col0, date_add(o_orderdate, interval o_orderkey%20000000 hour) as col1 from orders) select t1.o_orderkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    # 200w
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 2, 4) as col1 from part), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1 from customer) select t3.c_custkey, t3.col1, t6.col2 from (select t1.c_custkey, t1.col1 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.p_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.p_partkey;",
    # 200w
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 2, 4) as col1, substring(p_comment, 5, 4) as col2 from part), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1, substring(c_address, 5, 4) as col2 from customer) select t7.col0, t7.col1, t8.c_custkey from (select t3.c_custkey as col0, t3.col1 as col1, t6.col2 as col2 from (select t1.c_custkey, t2.col2 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.p_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.p_partkey) as t7 join cte2 as t8 on t7.col1 = t8.col2;",
]

tpch_sqls = [
    "with cte1 as (select S_SUPPKEY, S_SUPPKEY % 2000 as col0, length(S_COMMENT) as col1 from supplier) select t1.S_SUPPKEY, t1.col0, t1.col1 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 18, 10) as col1 from part) select t1.p_partkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    "with cte1 as (select ps_partkey, ps_suppkey, (ps_supplycost + ps_partkey) * 13 as col0, (ps_supplycost + ps_suppkey) * 13 as col1 from partsupp) select t1.ps_partkey, t1.ps_suppkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    "with cte1 as (select o_orderkey, date_add(o_orderdate, interval o_orderkey%10000000 hour) as col0, date_add(o_orderdate, interval o_orderkey%20000000 hour) as col1 from orders) select t1.o_orderkey, t1.col0 from cte1 t1 join cte1 t2 on t1.col0 = t2.col1;",
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 2, 4) as col1 from part), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1 from customer) select t3.c_custkey, t3.col1, t6.col2 from (select t1.c_custkey, t1.col1 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.p_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.p_partkey;",
    "with cte1 as (select p_partkey, substring(p_comment, 1, 20) as col0, substring(p_comment, 2, 4) as col1, substring(p_comment, 5, 4) as col2 from part), cte2 as (select c_custkey, substring(c_comment, 1, 20) as col0, substring(c_address, 1, 4) as col1, substring(c_address, 5, 4) as col2 from customer) select t7.col0, t7.col1, t8.c_custkey from (select t3.c_custkey as col0, t3.col1 as col1, t6.col2 as col2 from (select t1.c_custkey, t2.col2 as col1 from cte2 as t1 join cte1 as t2 on t1.col1 = t2.col1) as t3 join (select t4.p_partkey, t5.c_custkey, t4.col0 as col2 from cte1 as t4 join cte2 as t5 on t4.col0 = t5.col0) as t6 on t3.c_custkey = t6.p_partkey) as t7 join cte2 as t8 on t7.col1 = t8.col2;",
]

tpcds_sqls = [
    "WITH customer_total_return AS (SELECT sr_customer_sk AS ctr_customer_sk, sr_store_sk AS ctr_store_sk, Sum(sr_return_amt) AS ctr_total_return FROM store_returns, date_dim WHERE sr_returned_date_sk = d_date_sk AND d_year = 2001 GROUP BY sr_customer_sk, sr_store_sk) SELECT c_customer_id FROM customer_total_return ctr1, store, customer WHERE  ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_store_sk = ctr2.ctr_store_sk) AND s_store_sk = ctr1.ctr_store_sk AND s_state = 'TN' AND ctr1.ctr_customer_sk = c_customer_sk ORDER  BY c_customer_id LIMIT 100;",
    "WITH wscs as (select ws_sold_date_sk sold_date_sk ,ws_ext_sales_price sales_price from web_sales union all select cs_sold_date_sk sold_date_sk ,cs_ext_sales_price sales_price from catalog_sales), wswscs as (select d_week_seq, sum(case when (d_day_name='Sunday') then sales_price else null end) sun_sales, sum(case when (d_day_name='Monday') then sales_price else null end) mon_sales, sum(case when (d_day_name='Tuesday') then sales_price else null end) tue_sales, sum(case when (d_day_name='Wednesday') then sales_price else null end) wed_sales, sum(case when (d_day_name='Thursday') then sales_price else null end) thu_sales, sum(case when (d_day_name='Friday') then sales_price else null end) fri_sales, sum(case when (d_day_name='Saturday') then sales_price else null end) sat_sales from wscs ,date_dim where d_date_sk = sold_date_sk group by d_week_seq) select d_week_seq1 ,round(sun_sales1/sun_sales2,2) ,round(mon_sales1/mon_sales2,2) ,round(tue_sales1/tue_sales2,2) ,round(wed_sales1/wed_sales2,2) ,round(thu_sales1/thu_sales2,2) ,round(fri_sales1/fri_sales2,2) ,round(sat_sales1/sat_sales2,2) from (select wswscs.d_week_seq d_week_seq1 ,sun_sales sun_sales1 ,mon_sales mon_sales1 ,tue_sales tue_sales1 ,wed_sales wed_sales1 ,thu_sales thu_sales1 ,fri_sales fri_sales1 ,sat_sales sat_sales1 from wswscs,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001) y, (select wswscs.d_week_seq d_week_seq2 ,sun_sales sun_sales2 ,mon_sales mon_sales2 ,tue_sales tue_sales2 ,wed_sales wed_sales2 ,thu_sales thu_sales2 ,fri_sales fri_sales2 ,sat_sales sat_sales2 from wswscs ,date_dim where date_dim.d_week_seq = wswscs.d_week_seq and d_year = 2001+1) z where d_week_seq1=d_week_seq2-53 order by d_week_seq1;",
    "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(( ( ss_ext_list_price - ss_ext_wholesale_cost - ss_ext_discount_amt ) + ss_ext_sales_price ) / 2) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( cs_ext_list_price - cs_ext_wholesale_cost - cs_ext_discount_amt ) + cs_ext_sales_price ) / 2 )) year_total FROM customer, catalog_sales, date_dim WHERE c_customer_sk = cs_bill_customer_sk AND cs_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag, c_birth_country customer_birth_country , c_login customer_login, c_email_address customer_email_address , d_year dyear , Sum(( ( ( ws_ext_list_price - ws_ext_wholesale_cost - ws_ext_discount_amt ) + ws_ext_sales_price ) / 2 )) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_c_firstyear, year_total t_c_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_c_secyear.customer_id AND t_s_firstyear.customer_id = t_c_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_c_firstyear.dyear = 2001 AND t_c_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_c_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE NULL END AND CASE WHEN t_c_firstyear.year_total > 0 THEN t_c_secyear.year_total / t_c_firstyear.year_total ELSE NULL END > CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE NULL END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_preferred_cust_flag LIMIT 100;",
    "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ss_ext_list_price - ss_ext_discount_amt) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name , c_last_name customer_last_name, c_preferred_cust_flag customer_preferred_cust_flag , c_birth_country customer_birth_country, c_login customer_login, c_email_address customer_email_address, d_year dyear, Sum(ws_ext_list_price - ws_ext_discount_amt) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk GROUP BY c_customer_id, c_first_name, c_last_name, c_preferred_cust_flag, c_birth_country, c_login, c_email_address, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.dyear = 2001 AND t_s_secyear.dyear = 2001 + 1 AND t_w_firstyear.dyear = 2001 AND t_w_secyear.dyear = 2001 + 1 AND t_s_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE 0.0 END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE 0.0 END ORDER BY t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name, t_s_secyear.customer_birth_country LIMIT 100;",
    "WITH ss AS (SELECT ca_county, d_qoy, d_year, Sum(ss_ext_sales_price) AS store_sales FROM store_sales, date_dim, customer_address WHERE ss_sold_date_sk = d_date_sk AND ss_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year), ws AS (SELECT ca_county, d_qoy, d_year, Sum(ws_ext_sales_price) AS web_sales FROM web_sales, date_dim, customer_address WHERE ws_sold_date_sk = d_date_sk AND ws_bill_addr_sk = ca_address_sk GROUP BY ca_county, d_qoy, d_year) SELECT ss1.ca_county, ss1.d_year, ws2.web_sales / ws1.web_sales web_q1_q2_increase, ss2.store_sales / ss1.store_sales store_q1_q2_increase, ws3.web_sales / ws2.web_sales web_q2_q3_increase, ss3.store_sales / ss2.store_sales store_q2_q3_increase FROM ss ss1, ss ss2, ss ss3, ws ws1, ws ws2, ws ws3 WHERE ss1.d_qoy = 1 AND ss1.d_year = 2001 AND ss1.ca_county = ss2.ca_county AND ss2.d_qoy = 2 AND ss2.d_year = 2001 AND ss2.ca_county = ss3.ca_county AND ss3.d_qoy = 3 AND ss3.d_year = 2001 AND ss1.ca_county = ws1.ca_county AND ws1.d_qoy = 1 AND ws1.d_year = 2001 AND ws1.ca_county = ws2.ca_county AND ws2.d_qoy = 2 AND ws2.d_year = 2001 AND ws1.ca_county = ws3.ca_county AND ws3.d_qoy = 3 AND ws3.d_year = 2001 AND CASE WHEN ws1.web_sales > 0 THEN ws2.web_sales / ws1.web_sales ELSE NULL END > CASE WHEN ss1.store_sales > 0 THEN ss2.store_sales / ss1.store_sales ELSE NULL END AND CASE WHEN ws2.web_sales > 0 THEN ws3.web_sales / ws2.web_sales ELSE NULL END > CASE WHEN ss2.store_sales > 0 THEN ss3.store_sales / ss2.store_sales ELSE NULL END ORDER BY ss1.d_year;",
    "WITH wss AS (SELECT d_week_seq, ss_store_sk, Sum(CASE WHEN ( d_day_name = 'Sunday' ) THEN ss_sales_price ELSE NULL END) sun_sales, Sum(CASE WHEN ( d_day_name = 'Monday' ) THEN ss_sales_price ELSE NULL END) mon_sales, Sum(CASE WHEN ( d_day_name = 'Tuesday' ) THEN ss_sales_price ELSE NULL END) tue_sales, Sum(CASE WHEN ( d_day_name = 'Wednesday' ) THEN ss_sales_price ELSE NULL END) wed_sales, Sum(CASE WHEN ( d_day_name = 'Thursday' ) THEN ss_sales_price ELSE NULL END) thu_sales, Sum(CASE WHEN ( d_day_name = 'Friday' ) THEN ss_sales_price ELSE NULL END) fri_sales, Sum(CASE WHEN ( d_day_name = 'Saturday' ) THEN ss_sales_price ELSE NULL END) sat_sales FROM store_sales, date_dim WHERE d_date_sk = ss_sold_date_sk GROUP BY d_week_seq, ss_store_sk) SELECT s_store_name1, s_store_id1, d_week_seq1, sun_sales1 / sun_sales2, mon_sales1 / mon_sales2, tue_sales1 / tue_sales2, wed_sales1 / wed_sales2, thu_sales1 / thu_sales2, fri_sales1 / fri_sales2, sat_sales1 / sat_sales2 FROM (SELECT s_store_name s_store_name1, wss.d_week_seq d_week_seq1, s_store_id s_store_id1, sun_sales sun_sales1, mon_sales mon_sales1, tue_sales tue_sales1, wed_sales wed_sales1, thu_sales thu_sales1, fri_sales fri_sales1, sat_sales sat_sales1 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 AND 1196 + 11) y, (SELECT s_store_name s_store_name2, wss.d_week_seq d_week_seq2, s_store_id s_store_id2, sun_sales sun_sales2, mon_sales mon_sales2, tue_sales tue_sales2, wed_sales wed_sales2, thu_sales thu_sales2, fri_sales fri_sales2, sat_sales sat_sales2 FROM wss, store, date_dim d WHERE d.d_week_seq = wss.d_week_seq AND ss_store_sk = s_store_sk AND d_month_seq BETWEEN 1196 + 12 AND 1196 + 23) x WHERE s_store_id1 = s_store_id2 AND d_week_seq1 = d_week_seq2 - 52 ORDER BY s_store_name1, s_store_id1, d_week_seq1 LIMIT 100;",
    # Commented sql can not be pushed down to tiflash
    # "WITH cs_ui AS (SELECT cs_item_sk, Sum(cs_ext_list_price) AS sale, Sum(cr_refunded_cash + cr_reversed_charge + cr_store_credit) AS refund FROM catalog_sales, catalog_returns WHERE cs_item_sk = cr_item_sk AND cs_order_number = cr_order_number GROUP BY cs_item_sk HAVING Sum(cs_ext_list_price) > 2 * Sum( cr_refunded_cash + cr_reversed_charge + cr_store_credit)), cross_sales AS (SELECT i_product_name product_name, i_item_sk item_sk, s_store_name store_name, s_zip store_zip, ad1.ca_street_number b_street_number, ad1.ca_street_name b_streen_name, ad1.ca_city b_city, ad1.ca_zip b_zip, ad2.ca_street_number c_street_number, ad2.ca_street_name c_street_name, ad2.ca_city c_city, ad2.ca_zip c_zip, d1.d_year AS syear, d2.d_year AS fsyear, d3.d_year s2year, Count(*) cnt, Sum(ss_wholesale_cost) s1, Sum(ss_list_price) s2, Sum(ss_coupon_amt) s3 FROM store_sales, store_returns, cs_ui, date_dim d1, date_dim d2, date_dim d3, store, customer, customer_demographics cd1, customer_demographics cd2, promotion, household_demographics hd1, household_demographics hd2, customer_address ad1, customer_address ad2, income_band ib1, income_band ib2, item WHERE ss_store_sk = s_store_sk AND ss_sold_date_sk = d1.d_date_sk AND ss_customer_sk = c_customer_sk AND ss_cdemo_sk = cd1.cd_demo_sk AND ss_hdemo_sk = hd1.hd_demo_sk AND ss_addr_sk = ad1.ca_address_sk AND ss_item_sk = i_item_sk AND ss_item_sk = sr_item_sk AND ss_ticket_number = sr_ticket_number AND ss_item_sk = cs_ui.cs_item_sk AND c_current_cdemo_sk = cd2.cd_demo_sk AND c_current_hdemo_sk = hd2.hd_demo_sk AND c_current_addr_sk = ad2.ca_address_sk AND c_first_sales_date_sk = d2.d_date_sk AND c_first_shipto_date_sk = d3.d_date_sk AND ss_promo_sk = p_promo_sk AND hd1.hd_income_band_sk = ib1.ib_income_band_sk AND hd2.hd_income_band_sk = ib2.ib_income_band_sk AND cd1.cd_marital_status <> cd2.cd_marital_status AND i_color IN ( 'cyan', 'peach', 'blush', 'frosted', 'powder', 'orange' ) AND i_current_price BETWEEN 58 AND 58 + 10 AND i_current_price BETWEEN 58 + 1 AND 58 + 15 GROUP BY i_product_name, i_item_sk, s_store_name, s_zip, ad1.ca_street_number, ad1.ca_street_name, ad1.ca_city, ad1.ca_zip, ad2.ca_street_number, ad2.ca_street_name, ad2.ca_city, ad2.ca_zip, d1.d_year, d2.d_year, d3.d_year) SELECT cs1.product_name, cs1.store_name, cs1.store_zip, cs1.b_street_number, cs1.b_streen_name, cs1.b_city, cs1.b_zip, cs1.c_street_number, cs1.c_street_name, cs1.c_city, cs1.c_zip, cs1.syear, cs1.cnt, cs1.s1, cs1.s2, cs1.s3, cs2.s1, cs2.s2, cs2.s3, cs2.syear, cs2.cnt FROM cross_sales cs1, cross_sales cs2 WHERE cs1.item_sk = cs2.item_sk AND cs1.syear = 2001 AND cs2.syear = 2001 + 1 AND cs2.cnt <= cs1.cnt AND cs1.store_name = cs2.store_name AND cs1.store_zip = cs2.store_zip ORDER BY cs1.product_name, cs1.store_name, cs2.cnt;",
    # "WITH year_total AS (SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, d_year AS year1, Sum(ss_net_paid) year_total FROM customer, store_sales, date_dim WHERE c_customer_sk = ss_customer_sk AND ss_sold_date_sk = d_date_sk AND d_year IN ( 1999, 1999 + 1 ) GROUP BY c_customer_id, c_first_name, c_last_name, d_year UNION ALL SELECT c_customer_id customer_id, c_first_name customer_first_name, c_last_name customer_last_name, d_year AS year1, Sum(ws_net_paid) year_total FROM customer, web_sales, date_dim WHERE c_customer_sk = ws_bill_customer_sk AND ws_sold_date_sk = d_date_sk AND d_year IN ( 1999, 1999 + 1 ) GROUP BY c_customer_id, c_first_name, c_last_name, d_year) SELECT t_s_secyear.customer_id, t_s_secyear.customer_first_name, t_s_secyear.customer_last_name FROM year_total t_s_firstyear, year_total t_s_secyear, year_total t_w_firstyear, year_total t_w_secyear WHERE t_s_secyear.customer_id = t_s_firstyear.customer_id AND t_s_firstyear.customer_id = t_w_secyear.customer_id AND t_s_firstyear.customer_id = t_w_firstyear.customer_id AND t_s_firstyear.year1 = 1999 AND t_s_secyear.year1 = 1999 + 1 AND t_w_firstyear.year1 = 1999 AND t_w_secyear.year1 = 1999 + 1 AND t_s_firstyear.year_total > 0 AND t_w_firstyear.year_total > 0 AND CASE WHEN t_w_firstyear.year_total > 0 THEN t_w_secyear.year_total / t_w_firstyear.year_total ELSE NULL END > CASE WHEN t_s_firstyear.year_total > 0 THEN t_s_secyear.year_total / t_s_firstyear.year_total ELSE NULL END ORDER BY 1, 2, 3 LIMIT 100;",
    "WITH all_sales AS (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, Sum(sales_cnt) AS sales_cnt, Sum(sales_amt) AS sales_amt FROM (SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, cs_quantity - COALESCE(cr_return_quantity, 0) AS sales_cnt, cs_ext_sales_price - COALESCE(cr_return_amount, 0.0) AS sales_amt FROM catalog_sales JOIN item ON i_item_sk = cs_item_sk JOIN date_dim ON d_date_sk = cs_sold_date_sk LEFT JOIN catalog_returns ON ( cs_order_number = cr_order_number AND cs_item_sk = cr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ss_quantity - COALESCE(sr_return_quantity, 0) AS sales_cnt, ss_ext_sales_price - COALESCE(sr_return_amt, 0.0) AS sales_amt FROM store_sales JOIN item ON i_item_sk = ss_item_sk JOIN date_dim ON d_date_sk = ss_sold_date_sk LEFT JOIN store_returns ON ( ss_ticket_number = sr_ticket_number AND ss_item_sk = sr_item_sk ) WHERE i_category = 'Men' UNION SELECT d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id, ws_quantity - COALESCE(wr_return_quantity, 0) AS sales_cnt, ws_ext_sales_price - COALESCE(wr_return_amt, 0.0) AS sales_amt FROM web_sales JOIN item ON i_item_sk = ws_item_sk JOIN date_dim ON d_date_sk = ws_sold_date_sk LEFT JOIN web_returns ON ( ws_order_number = wr_order_number AND ws_item_sk = wr_item_sk ) WHERE i_category = 'Men') sales_detail GROUP BY d_year, i_brand_id, i_class_id, i_category_id, i_manufact_id) SELECT prev_yr.d_year AS prev_year, curr_yr.d_year AS year1, curr_yr.i_brand_id, curr_yr.i_class_id, curr_yr.i_category_id, curr_yr.i_manufact_id, prev_yr.sales_cnt AS prev_yr_cnt, curr_yr.sales_cnt AS curr_yr_cnt, curr_yr.sales_cnt - prev_yr.sales_cnt AS sales_cnt_diff, curr_yr.sales_amt - prev_yr.sales_amt AS sales_amt_diff FROM all_sales curr_yr, all_sales prev_yr WHERE curr_yr.i_brand_id = prev_yr.i_brand_id AND curr_yr.i_class_id = prev_yr.i_class_id AND curr_yr.i_category_id = prev_yr.i_category_id AND curr_yr.i_manufact_id = prev_yr.i_manufact_id AND curr_yr.d_year = 2002 AND prev_yr.d_year = 2002 - 1 AND Cast(curr_yr.sales_cnt AS DECIMAL(17, 2)) / Cast(prev_yr.sales_cnt AS DECIMAL(17, 2)) < 0.9 ORDER BY sales_cnt_diff LIMIT 100;",
    "WITH customer_total_return AS (SELECT cr_returning_customer_sk AS ctr_customer_sk, ca_state AS ctr_state, Sum(cr_return_amt_inc_tax) AS ctr_total_return FROM catalog_returns, date_dim, customer_address WHERE cr_returned_date_sk = d_date_sk AND d_year = 1999 AND cr_returning_addr_sk = ca_address_sk GROUP BY cr_returning_customer_sk, ca_state) SELECT c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return FROM customer_total_return ctr1, customer_address, customer WHERE ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_state = ctr2.ctr_state) AND ca_address_sk = c_current_addr_sk AND ca_state = 'TX' AND ctr1.ctr_customer_sk = c_customer_sk ORDER BY c_customer_id, c_salutation, c_first_name, c_last_name, ca_street_number, ca_street_name, ca_street_type, ca_suite_number, ca_city, ca_county, ca_state, ca_zip, ca_country, ca_gmt_offset, ca_location_type, ctr_total_return LIMIT 100; ## Q95 WITH ws_wh AS ( SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100;",
    "WITH ws_wh AS (SELECT ws1.ws_order_number, ws1.ws_warehouse_sk wh1, ws2.ws_warehouse_sk wh2 FROM web_sales ws1, web_sales ws2 WHERE ws1.ws_order_number = ws2.ws_order_number AND ws1.ws_warehouse_sk <> ws2.ws_warehouse_sk) SELECT Count(DISTINCT ws_order_number) AS `order count` , Sum(ws_ext_ship_cost) AS `total shipping cost` , Sum(ws_net_profit) AS `total net profit` FROM web_sales ws1 , date_dim , customer_address , web_site WHERE d_date BETWEEN '2000-4-01' AND ( Cast('2000-4-01' AS DATE) + INTERVAL '60' day) AND ws1.ws_ship_date_sk = d_date_sk AND ws1.ws_ship_addr_sk = ca_address_sk AND ca_state = 'IN' AND ws1.ws_web_site_sk = web_site_sk AND web_company_name = 'pri' AND ws1.ws_order_number IN ( SELECT ws_order_number FROM ws_wh) AND ws1.ws_order_number IN ( SELECT wr_order_number FROM web_returns, ws_wh WHERE wr_order_number = ws_wh.ws_order_number) ORDER BY count(DISTINCT ws_order_number) LIMIT 100;",
]

customized_sqls = [
    "with cte1 as (select col_key as col0, date_add(col3, interval 1 day) as col1, date_add(col3, interval 2 day) as col2 from ct1) select t1.col0, t1.col1 from cte1 as t1 join cte1 as t2 on t1.col1 = t2.col2;",
    "with cte1 as (select col_key, col1 + 10 as col0, date_add(col3, interval 1 day) as col1, substring(col2, 5, 5) as col2 from ct1), cte2 as (select col_key, col1 + 20 as col0, date_add(col3, interval 2 day) as col1, substring(col2, 10, 5) as col2 from ct2) select t3.col_key, t6.col2 from (select t1.col_key, t1.col2 from cte1 as t1 join cte2 as t2 on t1.col0 = t2.col0 ) as t3 join (select t4.col_key, t4.col2 from cte1 as t4 join cte2 as t5 on t4.col1 = t5.col1) as t6 on t3.col2 = t6.col2;",
    "with cte1 as (select col_key, col1 + 10 as col0, date_add(col3, interval 1 day) as col1, substring(col2, 5, 5) as col2, col1 + 100 as col3 from ct1), cte2 as (select col_key, col1 + 20 as col0, date_add(col3, interval 2 day) as col1, substring(col2, 10, 5) as col2 from ct2) select t3.col_key, t6.col2 from (select t1.col_key, t1.col2 from cte1 as t1 join cte1 as t2 on t1.col0 = t2.col3 ) as t3 join (select t4.col_key, t4.col2 from cte1 as t4 join cte2 as t5 on t4.col1 = t5.col1) as t6 on t3.col2 = t6.col2;",
    "with cte1 as (select col_key, col0 + 10 as col0, col0 + 20 as col1, col1 + 10.1 as col2, col1 + 1.1 as col3, substring(col2, 10, 5) as col4, substring(col2, 20, 5) as col5 from ct1) select t3.col_key, t6.col5 from (select t1.col_key, t1.col4 from cte1 as t1 join cte1 as t2 on t1.col0 = t2.col1) as t3 join (select t3.col5 from cte1 as t3 join cte1 as t4 on t3.col2 = t4.col3) as t6 on t3.col4 = t6.col5;",
    "with cte1 as (select col_key, col3 as col0, col1 as col1, col0 + 12 as col2, col0 + 34 as col3, substring(col2, 10, 5) as col4 from ct1), cte2 as (select col_key, col3 as col0, col1 as col1, date_add(col3, interval 10 day) as col2, date_add(col3, interval 20 day) as col3, substring(col2, 20, 5) as col4 from ct2) select t7.col_key, t7.col1, t10.col_key from ( select t3.col_key, t3.col1 from (select t2.col_key, t2.col2 as col0, t2.col4 as col1 from cte2 as t1 join cte1 as t2 on t1.col0 = t2.col0) as t3 join (select t4.col3 as col0 from cte1 as t4 join cte2 as t5 on t4.col1 = t5.col1) as t6 on t3.col0 = t6.col0 ) as t7 join ( select t8.col_key, t9.col4 from cte2 as t8 join cte2 as t9 on t8.col2 = t9.col3 ) as t10 on t7.col1 = t10.col4;",
    "with cte as (select col_key, col0 + 100 as col0, col0 + 1000 as col1, col1 + 1.1 as col2, col1 + 2.2 as col3, substring(col2, 20, 6) as col4, substring(col2, 30, 6) as col5, date_add(col3, interval 10 day) as col6, date_add(col3, interval 100 day) as col7, substring(col2, 40, 6) as col8, substring(col2, 45, 6) as col9 from ct2) select t7.col_key, t10.col_key, t7.col8 from ( select t3.col_key, t3.col8 from (select t1.col_key, t1.col4, t2.col8 from cte as t1 join cte as t2 on t1.col0 = t2.col1) as t3 join (select t4.col5 from cte as t4 join cte as t5 on t4.col2 = t5.col3) as t6 on t3.col4 = t6.col5 ) as t7 join (select t9.col_key, t8.col9 from cte as t8 join cte as t9 on t8.col6 = t9.col7) as t10 on t7.col8 = t10.col9;",
    "with cte as (select col_key as col0, col1 + 2 as col1 from ct1) select t1.col0, t1.col1, t2.col1 from (select col0, avg(col1) as col1 from cte group by col0) as t1 join (select col0, sum(col1) as col1 from cte group by col0) as t2 on t1.col0 = t2.col0;",
    #"with cte as (select col0, col1 from (select col_key as col0, col1 + 1.1 as col1 from ct1 union all select col_key as col0, col1 + 2.2 as col1 from ct2) as t1 union all select col_key as col0, col1 + 3.3 as col1 from ct2) select t3.col0, t3.col1 from cte as t3 join cte as t4 on t3.col1 = t4.col1;",
]

# databaseAndSqls = {"tpcds10": tpcds_sqls, "test": customized_sqls, "tpch10": tpch_sqls}
databaseAndSqls = {"tpcds100s": tpcds_spill_sqls}

def checkInTiflash(infos):
    find = False
    for item in infos:
        ret = re.search("CTEFullScan.*tiflash", str(item))
        if ret == None:
            continue
        find = True
        break
    return find

def compare(left, right):
    if len(left) != len(right):
        print("len(left) != len(right) %d %d" % (len(left, len(right))))
        raise Exception()
    
    length = len(left)
    i = 0
    while i < length:
        if left[i] != right[i]:
            print("Not equal: %s %s" % (left[i], right[i]))
            raise Exception()
        i += 1

def runWorker():
    global host
    global port
    global database
    global expect_results
    global sqls
    
    thread_name = threading.current_thread().name
    print("%s start at: %s" % (thread_name, datetime.now()))
    connection = pymysql.connect(host=host, port=port, user="root", database=database, cursorclass=pymysql.cursors.DictCursor)
    sql_num = len(sqls)

    with connection.cursor() as cursor:
        i = 0
        while i < 30:
            idx = random.randint(0, sql_num-1)
            sql = sqls[idx]
            print("%s starts to test sql %d" % (thread_name, idx))
            cursor.execute("set tidb_allow_mpp=1;")
            cursor.execute("set tidb_enforce_mpp=1;")
            cursor.execute(sql)
            sqlReturn = cursor.fetchall()
            result = []
            for oneRow in sqlReturn:
                result.append(str(oneRow))
            result.sort()
            print("%s starts to compare sql %d" % (thread_name, idx))
            compare(result, expect_results[idx])
            print("%s tests sql %d, and get expected result" % (thread_name, idx))
            i += 1

def runForConcurrentTest(target_func):
    global host
    global port
    global database
    global expect_results
    global sqls
    
    thread_num = 10
    
    print("Start Time: %s" % (datetime.now()))
    print("Host: %s, Port: %d, Database: %s, Test Time: %ds, Thread Num: %d" % (host, port, database, test_time, thread_num))

    expect_results = []
    connection = pymysql.connect(host=host, port=port, user="root", database=database, cursorclass=pymysql.cursors.DictCursor)
    print("Start to collect expected results")
    with connection:
        with connection.cursor() as cursor:
            i = 0
            for sql in sqls:
                print("Start to get expected result of sql %d" % i)
                cursor.execute("set tidb_allow_mpp=0;")
                cursor.execute(sql)
                sqlReturn = cursor.fetchall()
                result = []
                for oneRow in sqlReturn:
                    result.append(str(oneRow))
                result.sort()
                expect_results.append(result)
                i += 1
    print("Expected results have been collected")

    threads = []
    while (thread_num > 0):
        thread = threading.Thread(target=target_func)
        thread.start()
        threads.append(thread)
        thread_num -= 1

    start = time.time()
    for thread in threads:
        thread.join()
    end = time.time()
    print("Total time: %.2f" % (end - start))
    print("End Time: %s" % (datetime.now()))
    print("exit...")

def runTestForSingleSql():
    global host
    global port
    global databases
    global tpch_sqls
    global tpcds_sqls
    global customized_sqls

    print("Start Time: %s" % (datetime.now()))
    for database, sqls in databaseAndSqls.items():
        connection = pymysql.connect(host=host, port=port, user="root", database=database, cursorclass=pymysql.cursors.DictCursor)
        print("Host: %s, Port: %d, Database: %s" % (host, port, database))
        with connection:
            with connection.cursor() as cursor:
                i = 0
                print("Start to test for database %s" % database)
                for sql in sqls:
                    print("Start to get expected result of sql %d" % i)
                    cursor.execute("set tidb_mem_quota_query=40737418240;")
                    cursor.execute("set tidb_allow_mpp=0;")
                    cursor.execute("explain %s" % sql)
                    if checkInTiflash(cursor.fetchall()):
                        raise Exception("Sql %d is not executed in tidb" % i)
                    cursor.execute(sql)
                    sqlReturn = cursor.fetchall()
                    expected_result = []
                    for oneRow in sqlReturn:
                        expected_result.append(str(oneRow))
                    expected_result.sort()

                    print("Start to get actual result")
                    cursor.execute("set tidb_allow_mpp=1;")
                    cursor.execute("set tidb_enforce_mpp=1;")
                    cursor.execute("set tidb_opt_enable_mpp_shared_cte_execution=1;")
                    cursor.execute("set tidb_max_tiflash_threads=10")
                    cursor.execute("explain %s" % sql)
                    if not checkInTiflash(cursor.fetchall()):
                        raise Exception("Sql %d is not executed in tiflash" % i)
                    cursor.execute(sql)
                    sqlReturn = cursor.fetchall()
                    actual_result = []
                    for oneRow in sqlReturn:
                        actual_result.append(str(oneRow))
                    actual_result.sort()

                    print("Start to compare...")
                    compare(expected_result, actual_result)
                    print("Success!")
                    i += 1
    print("Test is finished. End time: %s" % datetime.now())

def runPerformance():
    host = "10.2.12.124"
    port = 8001
    perf_sqls = [
        "explain analyze WITH customer_total_return AS (SELECT sr_customer_sk AS ctr_customer_sk, sr_store_sk AS ctr_store_sk, Sum(sr_return_amt) AS ctr_total_return FROM store_returns, date_dim WHERE sr_returned_date_sk = d_date_sk AND d_year = 2001 GROUP BY sr_customer_sk, sr_store_sk) SELECT c_customer_id FROM customer_total_return ctr1, store, customer WHERE  ctr1.ctr_total_return > (SELECT Avg(ctr_total_return) * 1.2 FROM customer_total_return ctr2 WHERE ctr1.ctr_store_sk = ctr2.ctr_store_sk) AND s_store_sk = ctr1.ctr_store_sk AND s_state = 'TN' AND ctr1.ctr_customer_sk = c_customer_sk ORDER  BY c_customer_id LIMIT 100;"
    ]

    database = "tpcds100s"
    connection = pymysql.connect(host=host, port=port, user="root", database=database, cursorclass=pymysql.cursors.DictCursor)
    print("Host: %s, Port: %d, Database: %s" % (host, port, database))
    with connection:
        with connection.cursor() as cursor:
            cursor.execute("set tidb_allow_mpp=1;")
            cursor.execute("set tidb_enforce_mpp=1;")
            cursor.execute("set tidb_opt_enable_mpp_shared_cte_execution=1;")
            i = 0
            while True:
                print("run i %d, %s" % (i, datetime.now()))
                cursor.execute(perf_sqls[0])
                i += 1


if __name__ == "__main__":
    runTestForSingleSql()

