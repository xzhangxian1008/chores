STD_VERSION = -std=c++20
MACRO = -DMY_ENV
HEAD_FILES = util.h Stopwatch.h concurrent.h
OBJ_FILES = util.o main.o
SOURCE_FILES = util.cpp main.cpp
OPTIMIZE = -O0 -g
COMPILER = clang++
LINK_LIBS = -lpthread -lre2

all: m

m: main util 
	${COMPILER} ${OBJ_FILES} ${STD_VERSION} ${LINK_LIBS} ${-DMY_ENV} -o m ${OPTIMIZE}

main: main.cpp ${HEAD_FILES}
	${COMPILER} main.cpp -c ${OPTIMIZE} ${STD_VERSION}

util: util.cpp ${HEAD_FILES}
	${COMPILER} util.cpp -c ${OPTIMIZE} ${STD_VERSION}


E:
	${COMPILER} -E ${SOURCE_FILES}

S:
	${COMPILER} -S ${SOURCE_FILES}

