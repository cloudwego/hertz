namespace go toutiao.middleware.hertz

struct CommonType {
    1: required string IsCommonString;
    2: optional string TTT;
    3: required bool HHH;
    4: required Base GGG;
}

struct Base {
    1: optional string AAA;
    2: optional i32 BBB;
}