cmake_minimum_required (VERSION 3.21)
project(xzx)

message("xzx: ${xzx}")

option(flag " " OFF)

if (flag)
    message("enter if")
    option(xzx "xzx" OFF)
endif()

message("xzx: ${xzx}")
