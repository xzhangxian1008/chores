import string
import random

# varchar length should not be greater than 10
# number of number before decimal point should be less than 5. decimal(8, 3)
types = (["int", "bool", "decimal", "double", "varchar"])

def generateStr(length):
    return ''.join(random.choices(string.ascii_uppercase + string.digits, k=length))

def generateInts(num):
    ret = []
    i = 0
    while i < num:
        rand_num = random.randint(-9999, 9999)
        ret.append("%d" % rand_num)
        i += 1
    return ret

def generateBools(num):
    ret = []
    i = 0
    while i < num:
        rand_num = random.randint(-9, 8)
        if rand_num >= 0:
            ret.append("%s" % "true")
        else:
            ret.append("%s" % "false")
        i += 1
    return ret

def generateDecimals(num):
    ret = []
    i = 0
    while i < num:
        rand_num = random.uniform(-99999, 99999)
        ret.append("%.3f" % rand_num)
        i += 1
    return ret

def generateDoubles(num):
    ret = []
    i = 0
    while i < num:
        rand_num = random.uniform(-99999, 99999)
        ret.append("%f" % rand_num)
        i += 1
    return ret

def generateVarchars(num):
    ret = []
    i = 0
    while i < num:
        str_len = random.randint(1, 10)
        s = generateStr(str_len)
        ret.append("'%s'" % s)
        i += 1
    return ret

def generateByType(type, row_num, ndv):
    if row_num < ndv:
        raise Exception("row number shouldn't be smaller than ndv")
    if not type in types:
        raise Exception("Not Supported type")
    
    values = []
    if type == "int":
        values = generateInts(ndv)
    elif type == "bool":
        values = generateBools(ndv)
    elif type == "decimal":
        values = generateDecimals(ndv)
    elif type == "double":
        values = generateDoubles(ndv)
    else:
        values = generateVarchars(ndv)
    
    added_num = row_num - ndv
    i = 0
    while i < added_num:
        values.append(values[random.randrange(0, ndv)])
        i += 1
    return values

def generateInsertSqls(row_num, ndv, table_name, field_names_and_types):
    if len(field_names_and_types) < 1:
        raise Exception("length of field_names_and_types should be greater than 0")

    sql = "insert into %s" % table_name
    field_names = []
    values = []
    for name, type in field_names_and_types.items():
        field_names.append(name)
        values.append(generateByType(type, row_num, ndv))
    
    field_names_str = field_names[0]
    name_num = len(field_names)
    i = 1
    while i < name_num:
        field_names_str = "%s, %s" % (field_names_str, field_names[i])
        i += 1
    sql = "%s (%s) values" % (sql, field_names_str)

    column_num = len(values)
    i = 0
    while i < row_num:
        row_value = ""
        j = 0
        while j < column_num:
            if j > 0:
                row_value += ", "
            row_value += values[j][i]
            j += 1

        if i != 0:
            row_value = ", (%s)" % row_value
        else:
            row_value = "(%s)" % row_value
        sql += row_value
        i += 1

    return sql + ";"



if __name__ == "__main__":
    # a = {1:"a", 2:123}
    # for k, v in a.items():
    #     print(k, v)
    
    sql = generateInsertSqls(300, 200, "t", {"a": "int", "b":"decimal", "c":"double", "d":"varchar"})
    print(sql)
