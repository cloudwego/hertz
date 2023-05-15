include "basic_data.thrift"

namespace go toutiao.middleware.hertz_data

struct DataType {
    1: optional basic_data.BasicDataType IsDataString;
}