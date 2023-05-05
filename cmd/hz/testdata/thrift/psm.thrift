include "common.thrift"
include "data/data.thrift"

namespace go toutiao.middleware.hertz

const string STRING_CONST = "hertz";

enum EnumType {
    TWEET,
    RETWEET = 2,
}

typedef i32 MyInteger

struct BaseType {
    1: string GoTag = "test" (go.tag="json:\"go\" goTag:\"tag\"");
    2: optional string IsBaseString = "test";
    3: optional common.CommonType IsDepCommonType = {"IsCommonString":"test", "TTT":"test", "HHH":true, "GGG": {"AAA":"test","BBB":32}};
    4: optional EnumType IsBaseTypeEnum = 1;
}

typedef common.CommonType FFF

typedef BaseType MyBaseType

struct MultiTypeReq {
    // basic type (leading comments)
    1: optional bool IsBoolOpt = true; // trailing comments
    2: required bool IsBoolReq;
    3: optional byte IsByteOpt = 8;
    4: required byte IsByteReq;
    //5: optional i8 IsI8Opt; // unsupported i8, suggest byte
    //6: required i8 IsI8Req = 5; // default
    7: optional i16 IsI16Opt = 16;
    8: optional i32 IsI32Opt;
    9: optional i64 IsI64Opt;
    10: optional double IsDoubleOpt;
    11: required double IsDoubleReq;
    12: optional string IsStringOpt = "test";
    13: required string IsStringReq;

    14: optional list<string> IsList;
    22: required list<string> IsListReq;
    15: optional set<string> IsSet;
    16: optional map<string, string> IsMap;
    21: optional map<string, BaseType> IsStructMap;

    // struct type
    17: optional BaseType IsBaseType; // use struct name
    18: optional MyBaseType IsMyBaseType; // use typedef for struct
    19: optional common.CommonType IsCommonType = {"IsCommonString": "fffff"};
    20: optional data.DataType IsDataType; // multi-dependent struct
}

typedef data.DataType IsMyDataType

struct MultiTagReq {
    1: string QueryTag (api.query="query");
    2: string RawBodyTag (api.raw_body="raw_body");
    3: string PathTag (api.path="path");
    4: string FormTag (api.form="form");
    5: string CookieTag (api.cookie="cookie");
    6: string HeaderTag (api.header="header");
    7: string ProtobufTag (api.protobuf="protobuf");
    8: string BodyTag (api.body="body");
    9: string GoTag (go.tag="json:\"go\" goTag:\"tag\"");
    10: string VdTag (api.vd="$!='?'");
    11: string DefaultTag;
}

struct Resp {
    1: string Resp = "this is Resp";
}

struct MultiNameStyleReq {
  1: optional string hertz;
  2: optional string Hertz;
  3: optional string hertz_demo;
  4: optional string hertz_demo_idl;
  5: optional string hertz_Idl;
  6: optional string hertzDemo;
  7: optional string h;
  8: optional string H;
  9: optional string hertz_;
}

struct MultiDefaultReq {
  1: optional bool IsBoolOpt = true;
  2: required bool IsBoolReq = false;
  3: optional i32 IsI32Opt = 32;
  4: required i32 IsI32Req = 32;
  5: optional string IsStringOpt = "test";
  6: required string IsStringReq = "test";

  14: optional list<string> IsListOpt = ["test", "ttt", "sdsds"];
  22: required list<string> IsListReq = ["test", "ttt", "sdsds"];
  15: optional set<string> IsSet = ["test", "ttt", "sdsds"];
  16: optional map<string, string> IsMapOpt = {"test": "ttt", "ttt": "lll"};
  17: required map<string, string> IsMapReq = {"test": "ttt", "ttt": "lll"};
  21: optional map<string, BaseType> IsStructMapOpt = {"test": {"GoTag":"fff", "IsBaseTypeEnum":1, "IsBaseString":"ddd", "IsDepCommonType": {"IsCommonString":"fffffff", "TTT":"ttt", "HHH":true, "GGG": {"AAA":"test","BBB":32}}}};
  25: required map<string, BaseType> IsStructMapReq = {"test": {"GoTag":"fff", "IsBaseTypeEnum":1, "IsBaseString":"ddd", "IsDepCommonType": {"IsCommonString":"fffffff", "TTT":"ttt", "HHH":true, "GGG": {"AAA":"test","BBB":32}}}};

  23: optional common.CommonType IsDepCommonTypeOpt = {"IsCommonString":"fffffff", "TTT":"ttt", "HHH":true, "GGG": {"AAA":"test","BBB":32}};
  24: required common.CommonType IsDepCommonTypeReq = {"IsCommonString":"fffffff", "TTT":"ttt", "HHH":true, "GGG": {"AAA":"test","BBB":32}};
}

typedef map<string, string> IsTypedefContainer

service Hertz {
    Resp Method1(1: MultiTypeReq request) (api.get="/company/department/group/user:id/name", api.handler_path="v1");
    Resp Method2(1: MultiTagReq request) (api.post="/company/department/group/user:id/sex", api.handler_path="v1");
    Resp Method3(1: BaseType request) (api.put="/company/department/group/user:id/number", api.handler_path="v1");
    Resp Method4(1: data.DataType request) (api.delete="/company/department/group/user:id/age", api.handler_path="v1");

    Resp Method5(1: MultiTypeReq request) (api.options="/school/class/student/name", api.handler_path="v2");
    Resp Method6(1: MultiTagReq request) (api.head="/school/class/student/number", api.handler_path="v2");
    Resp Method7(1: MultiTagReq request) (api.patch="/school/class/student/sex", api.handler_path="v2");
    Resp Method8(1: BaseType request) (api.any="/school/class/student/grade/*subjects", api.handler_path="v2");

    Resp Method9(1: IsTypedefContainer request) (api.get="/typedef/container", api.handler_path="v2");
    Resp Method10(1:  map<string, string> request) (api.get="/container", api.handler_path="v2");
}