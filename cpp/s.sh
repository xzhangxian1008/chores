#!/bin/bash

if [ $1 = "m" ]
then
    make m -j 6
    LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH ./main
elif [ $1 = "server" ]
then
    make server -j 6
    LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH ./ser
elif [ $1 = "client" ]
then
    make client -j 6
    LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH ./cli
else
    echo "The compiler is else"
fi


# make -j 4
# LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH ./m
