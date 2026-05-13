def my_ts_library(name, srcs, **kwargs):
    native.filegroup(name = name, srcs = srcs, **kwargs)

def my_ts_test(name, srcs, deps = [], data = [], **kwargs):
    native.filegroup(name = name, srcs = srcs + deps + data, **kwargs)
