import re

class GoroutineStackInfo:
    def __init__(self, id):
        self.id = id
        self.content = ""
    
    def addContent(self, content):
        self.content += content
    
    def printInfo(self):
        print(self.content)
    
def collectGoroutinesInfo(filename):
    stackInfo = {}
    currentID = 0

    with open(filename, "r") as f:
        lines = f.readlines()
        for line in lines:
            res = re.match("goroutine", line)
            if res != None:
                currentID = getGoroutineID(line)
                stackInfo[currentID] = GoroutineStackInfo(currentID)

            stackInfo[currentID].addContent(line)
    
    return stackInfo

def getGoroutineID(line):
    num = 0
    line = line[10:]
    for c in line:
        if c == ' ':
            return num
        
        num *= 10
        num += int(c)

def checkLeak(file1, file2):
    old = collectGoroutinesInfo(file1)
    new = collectGoroutinesInfo(file2)
    for key, value in new.items():
        if key not in old:
            value.printInfo()

if __name__ == "__main__":
    checkLeak("file1", "file2")
