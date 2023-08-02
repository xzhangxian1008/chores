import threading
import tpch

class Sql:
    def __init__(self, sql, sql_name):
        self.sql = sql
        self.sql_name = sql_name
        self.total_time = 0
        self.total_count = 0
        self.lock = threading.Lock()

    def getSqlName(self):
        return self.sql_name

    def getSql(self):
        return self.sql

    def addInfo(self, executed_time):
        self.lock.acquire()
        self.total_time += executed_time
        self.total_count += 1
        self.lock.release()

    def getAvgTime(self):
        self.lock.acquire()
        avg = 0
        if self.total_count != 0:
            avg = self.total_time / self.total_count
        self.lock.release()
        return avg

    def getCount(self):
        self.lock.acquire()
        count = self.total_count
        self.lock.release()
        return count

    def getInfo(self):
        info = "%s[avg time: %fs, total count:%d]" % (self.sql_name, self.getAvgTime(), self.getCount())
        return info


class SpillQueryInfo:
    def __init__(self, query, support_join_spill, join_spill_threshold, support_agg_spill, agg_spill_threshold, support_sort_spill, sort_spill_threshold):
        self.query = query # Sql
        self.support_join_spill = support_join_spill # bool
        self.join_spill_threshold = join_spill_threshold # int
        self.support_agg_spill = support_agg_spill # bool
        self.agg_spill_threshold = agg_spill_threshold # int
        self.support_sort_spill = support_sort_spill # bool
        self.sort_spill_threshold = sort_spill_threshold # int

    # The name of this function must be the same with Sql's
    def getExecutedSql(self, all_param_no_default):
        if all_param_no_default:
            return """set tidb_max_bytes_before_tiflash_external_join=%d;
                      set tidb_max_bytes_before_tiflash_external_group_by=%d;
                      set tidb_max_bytes_before_tiflash_external_sort=%d;
                      %s""" % (self.join_spill_threshold, self.agg_spill_threshold, self.sort_spill_threshold, self.query.getSql())

        # case 1, without spill
        queries = """set tidb_max_bytes_before_tiflash_external_join=0;
                      set tidb_max_bytes_before_tiflash_external_group_by=0;
                      set tidb_max_bytes_before_tiflash_external_sort=0;
                      %s""" % (self.query.getSql())

        query_num = 1

        # case 2, join spill only
        if self.support_join_spill:
            query_num += 1
            queries += """set tidb_max_bytes_before_tiflash_external_join=%d;
                          set tidb_max_bytes_before_tiflash_external_group_by=0;
                          set tidb_max_bytes_before_tiflash_external_sort=0;
                          %s""" % (self.join_spill_threshold, self.query.getSql())

        # case 3, agg spill only
        if self.support_agg_spill:
            query_num += 1
            queries += """set tidb_max_bytes_before_tiflash_external_join=0;
                          set tidb_max_bytes_before_tiflash_external_group_by=%d;
                          set tidb_max_bytes_before_tiflash_external_sort=0;
                          %s""" % (self.agg_spill_threshold, self.query.getSql())

        # case 4, sort spill only
        if self.support_sort_spill:
            query_num += 1
            queries += """set tidb_max_bytes_before_tiflash_external_join=0;
                      set tidb_max_bytes_before_tiflash_external_group_by=0;
                      set tidb_max_bytes_before_tiflash_external_sort=%d;
                      %s""" % (self.sort_spill_threshold, self.query.getSql())

        if query_num > 2:
            query_num += 1
            queries += """set tidb_max_bytes_before_tiflash_external_join=%d;
                          set tidb_max_bytes_before_tiflash_external_group_by=%d;
                          set tidb_max_bytes_before_tiflash_external_sort=%d;
                          %s""" % (self.join_spill_threshold, self.agg_spill_threshold, self.sort_spill_threshold, self.query.getSql())

        return queries


tpch_sqls = {
    1: Sql("%s;" % tpch.q1, "Q1"),
    2: Sql("%s;" % tpch.q2, "Q2"),
    3: Sql("%s;" % tpch.q3, "Q3"),
    4: Sql("%s;" % tpch.q4, "Q4"),
    5: Sql("%s;" % tpch.q5, "Q5"),
    6: Sql("%s;" % tpch.q6, "Q6"),
    7: Sql("%s;" % tpch.q7, "Q7"),
    8: Sql("%s;" % tpch.q8, "Q8"),
    9: Sql("%s;" % tpch.q9, "Q9"),
    10: Sql("%s;" % tpch.q10, "Q10"),
    11: Sql("%s;" % tpch.q11, "Q11"),
    12: Sql("%s;" % tpch.q12, "Q12"),
    13: Sql("%s;" % tpch.q13, "Q13"),
    14: Sql("%s;" % tpch.q14, "Q14"),
    15: Sql("%s;" % tpch.q15, "Q15"),
    16: Sql("%s;" % tpch.q16, "Q16"),
    17: Sql("%s;" % tpch.q17, "Q17"),
    18: Sql("%s;" % tpch.q18, "Q18"),
    19: Sql("%s;" % tpch.q19, "Q19"),
    20: Sql("%s;" % tpch.q20, "Q20"),
    21: Sql("%s;" % tpch.q21, "Q21"),
    22: Sql("%s;" % tpch.q22, "Q22")
}

# for tpch1 and 2 tiflash nodes
tpch1_spill_infos = {
    1: SpillQueryInfo(Sql(tpch.q1, "Q1"), False, 0, False, 0, False, 0),
    2: SpillQueryInfo(Sql(tpch.q2, "Q2"), True, 100 * 1024, True, 1, True, 100),
    3: SpillQueryInfo(Sql(tpch.q3, "Q3"), True, 1024 * 1024, True, 1024 * 1024, True, 100),
    4: SpillQueryInfo(Sql(tpch.q4, "Q4"), True, 3 * 1024 * 1024, True, 10 * 1024, False, 0),
    5: SpillQueryInfo(Sql(tpch.q5, "Q5"), True, 30 * 1024 * 1024, True, 10 * 1024, False, 0),
    6: SpillQueryInfo(Sql(tpch.q6, "Q6"), False, 0, False, 0, False, 0),
    7: SpillQueryInfo(Sql(tpch.q7, "Q7"), True, 10 * 1024 * 1024, False, 0, False, 0),
    8: SpillQueryInfo(Sql(tpch.q8, "Q8"), True, 0 * 1024 * 1024, False, 0, False, 0),
    9: SpillQueryInfo(Sql(tpch.q9, "Q9"), True, 20 * 1024 * 1024, False, 0, False, 0),
    10: SpillQueryInfo(Sql(tpch.q10, "Q10"), True, 10 * 1024 * 1024, True, 100 * 1024, True, 100),
    11: SpillQueryInfo(Sql(tpch.q11, "Q11"), True, 100, False, 0, False, 0),
    12: SpillQueryInfo(Sql(tpch.q12, "Q12"), True, 1024 * 1024, False, 0, False, 0),
    13: SpillQueryInfo(Sql(tpch.q13, "Q13"), True, 5 * 1024 * 1024, False, 0, False, 0),
    14: SpillQueryInfo(Sql(tpch.q14, "Q14"), True, 5 * 1024 * 1024, False, 0, False, 0),
    15: SpillQueryInfo(Sql(tpch.q15, "Q15"), True, 100, False, 0, False, 0),
    16: SpillQueryInfo(Sql(tpch.q16, "Q16"), True, 1024 * 1024, True, 100 * 1024, False, 0),
    17: SpillQueryInfo(Sql(tpch.q17, "Q17"), True, 100 * 1024, True, 1024 * 1024, False, 0),
    18: SpillQueryInfo(Sql(tpch.q18, "Q18"), True, 1024 * 1024, True, 100 * 1024, True, 100),
    19: SpillQueryInfo(Sql(tpch.q19, "Q19"), True, 10 * 1024, False, 0, False, 0),
    20: SpillQueryInfo(Sql(tpch.q20, "Q20"), True, 500 * 1024, True, 100 * 1024, False, 0),
    21: SpillQueryInfo(Sql(tpch.q21, "Q21"), True, 20 * 1024 * 1024, True, 100 * 1024, True, 100),
    22: SpillQueryInfo(Sql(tpch.q22, "Q22"), True, 200 * 1024, False, 0, False, 0)
}


# for tpch10 and 2 tiflash nodes
tpch10_spill_infos = {
    1: SpillQueryInfo(Sql(tpch.q1, "Q1"), False, 0, False, 0, False, 0),
    2: SpillQueryInfo(Sql(tpch.q2, "Q2"), True, 1024 * 1024, True, 1, True, 1024),
    3: SpillQueryInfo(Sql(tpch.q3, "Q3"), True, 10 * 1024 * 1024, True, 10 * 1024 * 1024, True, 1024),
    4: SpillQueryInfo(Sql(tpch.q4, "Q4"), True, 30 * 1024 * 1024, True, 128 * 1024, False, 0),
    5: SpillQueryInfo(Sql(tpch.q5, "Q5"), True, 350 * 1024 * 1024, True, 128 * 1024, False, 0),
    6: SpillQueryInfo(Sql(tpch.q6, "Q6"), False, 0, False, 0, False, 0),
    7: SpillQueryInfo(Sql(tpch.q7, "Q7"), True, 100 * 1024 * 1024, False, 0, False, 0),
    8: SpillQueryInfo(Sql(tpch.q8, "Q8"), True, 50 * 1024 * 1024, False, 0, False, 0),
    9: SpillQueryInfo(Sql(tpch.q9, "Q9"), True, 200 * 1024 * 1024, False, 0, False, 0),
    10: SpillQueryInfo(Sql(tpch.q10, "Q10"), True, 100 * 1024 * 1024, True, 1024 * 1024, True, 1024),
    11: SpillQueryInfo(Sql(tpch.q11, "Q11"), True, 1024, False, 0, False, 0),
    12: SpillQueryInfo(Sql(tpch.q12, "Q12"), True, 10 * 1024 * 1024, False, 0, False, 0),
    13: SpillQueryInfo(Sql(tpch.q13, "Q13"), True, 50 * 1024 * 1024, False, 0, False, 0),
    14: SpillQueryInfo(Sql(tpch.q14, "Q14"), True, 50 * 1024 * 1024, False, 0, False, 0),
    15: SpillQueryInfo(Sql(tpch.q15, "Q15"), True, 1024, False, 0, False, 0),
    16: SpillQueryInfo(Sql(tpch.q16, "Q16"), True, 10 * 1024 * 1024, True, 1024 * 1024, False, 0),
    17: SpillQueryInfo(Sql(tpch.q17, "Q17"), True, 1024 * 1024, True, 10 * 1024 * 1024, False, 0),
    18: SpillQueryInfo(Sql(tpch.q18, "Q18"), True, 10 * 1024 * 1024, True, 1024 * 1024, True, 1024),
    19: SpillQueryInfo(Sql(tpch.q19, "Q19"), True, 128 * 1024, False, 0, False, 0),
    20: SpillQueryInfo(Sql(tpch.q20, "Q20"), True, 5 * 1024 * 1024, True, 1024 * 1024, False, 0),
    21: SpillQueryInfo(Sql(tpch.q21, "Q21"), True, 30 * 1024 * 1024, True, 1024 * 1024, True, 1024),
    22: SpillQueryInfo(Sql(tpch.q22, "Q22"), True, 2 * 1024 * 1024, False, 0, False, 0)
}

tpch1_spill_sqls = {
    1: Sql(tpch1_spill_infos[1].getExecutedSql(False), "Q1"),
    2: Sql(tpch1_spill_infos[2].getExecutedSql(False), "Q2"),
    3: Sql(tpch1_spill_infos[3].getExecutedSql(False), "Q3"),
    4: Sql(tpch1_spill_infos[4].getExecutedSql(False), "Q4"),
    5: Sql(tpch1_spill_infos[5].getExecutedSql(False), "Q5"),
    6: Sql(tpch1_spill_infos[6].getExecutedSql(False), "Q6"),
    7: Sql(tpch1_spill_infos[7].getExecutedSql(False), "Q7"),
    8: Sql(tpch1_spill_infos[8].getExecutedSql(False), "Q8"),
    9: Sql(tpch1_spill_infos[9].getExecutedSql(False), "Q9"),
    10: Sql(tpch1_spill_infos[10].getExecutedSql(False), "Q10"),
    11: Sql(tpch1_spill_infos[11].getExecutedSql(False), "Q11"),
    12: Sql(tpch1_spill_infos[12].getExecutedSql(False), "Q12"),
    13: Sql(tpch1_spill_infos[13].getExecutedSql(False), "Q13"),
    14: Sql(tpch1_spill_infos[14].getExecutedSql(False), "Q14"),
    15: Sql(tpch1_spill_infos[15].getExecutedSql(False), "Q15"),
    16: Sql(tpch1_spill_infos[16].getExecutedSql(False), "Q16"),
    17: Sql(tpch1_spill_infos[17].getExecutedSql(False), "Q17"),
    18: Sql(tpch1_spill_infos[18].getExecutedSql(False), "Q18"),
    19: Sql(tpch1_spill_infos[19].getExecutedSql(False), "Q19"),
    20: Sql(tpch1_spill_infos[20].getExecutedSql(False), "Q20"),
    21: Sql(tpch1_spill_infos[21].getExecutedSql(False), "Q21"),
    22: Sql(tpch1_spill_infos[22].getExecutedSql(False), "Q22"),
}

tpch10_spill_sqls = {
    1: Sql(tpch10_spill_infos[1].getExecutedSql(False), "Q1"),
    2: Sql(tpch10_spill_infos[2].getExecutedSql(False), "Q2"),
    3: Sql(tpch10_spill_infos[3].getExecutedSql(False), "Q3"),
    4: Sql(tpch10_spill_infos[4].getExecutedSql(False), "Q4"),
    5: Sql(tpch10_spill_infos[5].getExecutedSql(False), "Q5"),
    6: Sql(tpch10_spill_infos[6].getExecutedSql(False), "Q6"),
    7: Sql(tpch10_spill_infos[7].getExecutedSql(False), "Q7"),
    8: Sql(tpch10_spill_infos[8].getExecutedSql(False), "Q8"),
    9: Sql(tpch10_spill_infos[9].getExecutedSql(False), "Q9"),
    10: Sql(tpch10_spill_infos[10].getExecutedSql(False), "Q10"),
    11: Sql(tpch10_spill_infos[11].getExecutedSql(False), "Q11"),
    12: Sql(tpch10_spill_infos[12].getExecutedSql(False), "Q12"),
    13: Sql(tpch10_spill_infos[13].getExecutedSql(False), "Q13"),
    14: Sql(tpch10_spill_infos[14].getExecutedSql(False), "Q14"),
    15: Sql(tpch10_spill_infos[15].getExecutedSql(False), "Q15"),
    16: Sql(tpch10_spill_infos[16].getExecutedSql(False), "Q16"),
    17: Sql(tpch10_spill_infos[17].getExecutedSql(False), "Q17"),
    18: Sql(tpch10_spill_infos[18].getExecutedSql(False), "Q18"),
    19: Sql(tpch10_spill_infos[19].getExecutedSql(False), "Q19"),
    20: Sql(tpch10_spill_infos[20].getExecutedSql(False), "Q20"),
    21: Sql(tpch10_spill_infos[21].getExecutedSql(False), "Q21"),
    22: Sql(tpch10_spill_infos[22].getExecutedSql(False), "Q22"),
}
