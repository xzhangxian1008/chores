def getNumbers(s):
    results = s.split("	")
    return int(results[0]), int(results[1])

if __name__ == "__main__":
    prevCol0 = -1
    prevCol1 = 0

    with open("output.txt", "r") as f:
        file = f.readlines()
        for line in file:
            col0, col1 = getNumbers(line)
            if col0 < prevCol0:
                raise Exception("col0: %d, prevCol0: %d" % (col0, prevCol0))
            if col1 != prevCol1+1:
                raise Exception("col1: %d, prevCol1: %d" % (col1, prevCol1))
            prevCol0 = col0
            prevCol1 = col1
