import random

# [start, end]
def generateRandomInt(start, end):
    return random.randint(start, end)

def generateValue():
    return "(%d, %d, %d)" % (generateRandomInt(-50000, 50000), generateRandomInt(-100000, 100000), generateRandomInt(-100000, 100000))
