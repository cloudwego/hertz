namespace go cloudwego.hertz.hz

struct MultiTagReq {
    // basic feature
    1: string DefaultQueryTag (api.query="query");
    2: string RawBodyTag (api.raw_body="raw_body");
    3: string PathTag (api.path="path");
    4: string FormTag (api.form="form");
    5: string CookieTag (api.cookie="cookie");
    6: string HeaderTag (api.header="header");
    7: string BodyTag (api.body="body");
    8: string GoTag (go.tag="json:\"json\" query:\"query\" form:\"form\" header:\"header\" goTag:\"tag\"");
    9: string VdTag (api.vd="$!='?'");
    10: string DefaultTag;

    // optional / required
    11: required string ReqQuery (api.query="query");
    12: optional string OptQuery (api.query="query");
    13: required string ReqBody (api.body="body");
    14: optional string OptBody (api.body="body");
    15: required string ReqGoTag (go.tag="json:\"json\"");
    16: optional string OptGoTag (go.tag="json:\"json\"");

    // gotag cover feature
    17: required string QueryGoTag (apt.query="query", go.tag="query:\"queryTag\"")
}