if __name__ == "__main__":
    file1 = []
    file2 = []
    with open("res1.txt", "r") as f:
        file1 = f.readlines()

    with open("res2.txt", "r") as f:
        file2 = f.readlines()

    if len(file1) != len(file2):
        raise Exception("length not equal, %d vs %d" % (len(file1), len(file2)))

    file1.sort()
    file2.sort()
    length = len(file1)
    i = 0
    while i < length:
        if file1[i] != file2[i]:
            print("error: i %d, <%s> vs <%s>" % (i, file1[i], file2[i]))
            raise Exception("Incorrect answer")
        i += 1

    print("Success!")