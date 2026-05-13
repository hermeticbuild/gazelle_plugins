def my_ts_library(name, srcs, **kwargs):
    native.filegroup(name = name, srcs = srcs, **kwargs)

def my_ts_test(name, data, **kwargs):
    native.filegroup(name = name, srcs = data, **kwargs)
