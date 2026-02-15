STD_VERSION = -std=c++20

COMMON_HEAD_FILES = util.h
COMMON_OBJ_FILES = util.o

MAIN_HEAD_FILES = Stopwatch.h concurrent.h bench.h
MAIN_OBJ_FILES = main.o bench.o
MAIN_BINARY_FILE = main

SERVER_OBJ_FILES = server.o
SERVER_BINARY_FILE = server

CLIENT_OBJ_FILES = client.o
CLIENT_BINARY_FILE = client

OPTIMIZE = -O0 -g
# OPTIMIZE = -O3
COMPILER = clang++
LINK_LIBS = -lpthread -lre2 -lrocksdb

test:
	echo ${MAIN_HEAD_FILES}

server: serv util
	${COMPILER} ${COMMON_OBJ_FILES} ${SERVER_OBJ_FILES} ${STD_VERSION} ${LINK_LIBS} -o ${SERVER_BINARY_FILE} ${OPTIMIZE}

client: cli util
	${COMPILER} ${COMMON_OBJ_FILES} ${SERVER_OBJ_FILES} ${STD_VERSION} ${LINK_LIBS} -o ${CLIENT_BINARY_FILE} ${OPTIMIZE}

main: m util bench
	${COMPILER} ${MAIN_OBJ_FILES} ${COMMON_OBJ_FILES} ${STD_VERSION} ${LINK_LIBS} -o ${MAIN_BINARY_FILE} ${OPTIMIZE}

m: main.cpp ${MAIN_HEAD_FILES}
	${COMPILER} main.cpp -c ${OPTIMIZE} ${STD_VERSION} ${INCLUDE_PATH} 

serv: server.cpp
	${COMPILER} server.cpp -c ${OPTIMIZE} ${STD_VERSION} ${INCLUDE_PATH} 

cli: client.cpp
	${COMPILER} client.cpp -c ${OPTIMIZE} ${STD_VERSION} ${INCLUDE_PATH} 

util: util.cpp util.h
	${COMPILER} util.cpp -c ${OPTIMIZE} ${STD_VERSION}

bench: bench.cpp ${HEAD_FILES}
	${COMPILER} bench.cpp -c ${OPTIMIZE} ${STD_VERSION}

clean:
	rm ${MAIN_OBJ_FILES} ${COMMON_OBJ_FILES} ${SERVER_OBJ_FILES} ${CLIENT_OBJ_FILES} ${MAIN_BINARY_FILE} ${SERVER_BINARY_FILE} ${CLIENT_BINARY_FILE}
