STD_VERSION = -std=c++20
MACRO = -DMY_ENV
HEAD_FILES = util.h Stopwatch.h concurrent.h bench.h
OBJ_FILES = util.o main.o bench.o
SOURCE_FILES = util.cpp main.cpp bench.cpp
# OPTIMIZE = -O0 -g
OPTIMIZE = -O3
COMPILER = clang++
LINK_LIBS = -lpthread -lre2 -lleveldb
# LINK_LIBS = -lpthread

all: m

m: main util bench
	${COMPILER} ${OBJ_FILES} ${STD_VERSION} ${LINK_LIBS} ${-DMY_ENV} -o m ${OPTIMIZE}

main: main.cpp ${HEAD_FILES}
	${COMPILER} main.cpp -c ${OPTIMIZE} ${STD_VERSION}

util: util.cpp ${HEAD_FILES}
	${COMPILER} util.cpp -c ${OPTIMIZE} ${STD_VERSION}

bench: bench.cpp ${HEAD_FILES}
	${COMPILER} bench.cpp -c ${OPTIMIZE} ${STD_VERSION}

E:
	${COMPILER} -E ${SOURCE_FILES}

S:
	${COMPILER} -S ${SOURCE_FILES}

