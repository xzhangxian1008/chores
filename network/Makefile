STD_VERSION = -std=c++20
MACRO = -DMY_ENV
HEAD_FILES = util.h
OBJ_FILES = server.o client.o
SOURCE_FILES = server.cpp client.cpp
# OPTIMIZE = -O0 -g -fomit-frame-pointer
OPTIMIZE = -O3
COMPILER = clang++
LINK_LIBS = -lpthread

all: m

m: server client

server: server.cpp ${HEAD_FILES}
	${COMPILER} server.cpp ${OPTIMIZE} ${STD_VERSION} -o server

client: client.cpp ${HEAD_FILES}
	${COMPILER} client.cpp ${OPTIMIZE} ${STD_VERSION} -o client

E:
	${COMPILER} -E ${SOURCE_FILES}

S:
	${COMPILER} -S ${SOURCE_FILES}

