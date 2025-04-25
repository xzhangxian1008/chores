import re
import os

pre_sql = "test> "

select_pattern = "select.*"
insert_pattern = "insert into.*"
drop_table_pattern = "drop table if exists.*"
create_table_pattern = "create table.*"
drop_database_pattern = "drop database if exists.*"
create_database_pattern = "create database.*"

patterns = [select_pattern, insert_pattern, drop_table_pattern, create_table_pattern, drop_database_pattern, create_database_pattern]

def processOneLine(line):
    for pattern in patterns:
        match = re.search(pattern, line, re.IGNORECASE)
        if match != None:
            span = match.span()
            return line[span[0]: span[1]]
    return ""

def processOneFile(input_name, output_name):
    lines = []
    with open(input_name, 'r') as f:
        lines = f.readlines()
    
    with open(output_name, 'w') as f:
        for line in lines:
            sql = processOneLine(line)
            if sql != "":
                f.write("%s\n" % sql)

def run():
    input_files = []
    output_files = []
    all_files = os.listdir()
    for file in all_files:
        match = re.search(".*py", file)
        if os.path.isfile(file) and match == None:
            input_files.append(file)
            output_files.append("output_%s" % file)

    file_num = len(input_files)
    i = 0
    while i < file_num:
        processOneFile(input_files[i], output_files[i])
        i += 1

if __name__ == "__main__":
    run()
