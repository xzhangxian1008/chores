def getResultMap(result_file):
    result = {}
    with open(result_file, 'r') as f:
        results = f.readlines()
        for res in results:
            if res in result:
                result[res] += 1
            else:
                result[res] = 1
    return result

def compare(result1, result2):
    if len(result1) != len(result2):
        return False

    for key in result1:
        if key in result2:
            if result1[key] != result2[key]:
                return False
        else:
            return False
    return True

def compareResults(result1_file, result2_file):
    result1 = getResultMap(result1_file)
    result2 = getResultMap(result2_file)
    return compare(result1, result2)

if __name__ == "__main__":
    if compareResults("spill", "no-spill"):
        print("PASS")
    else:
        print("FAIL")
