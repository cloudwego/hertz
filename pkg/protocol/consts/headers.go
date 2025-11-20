/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package consts

const (
	HeaderDate = "Date"

	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderLastModified    = "Last-Modified"

	// Redirects
	HeaderLocation = "Location"

	// Transfer coding
	HeaderTE               = "TE"
	HeaderTrailer          = "Trailer"
	HeaderTrailerLower     = "trailer"
	HeaderTransferEncoding = "Transfer-Encoding"

	// Controls
	HeaderCookie         = "Cookie"
	HeaderExpect         = "Expect"
	HeaderMaxForwards    = "Max-Forwards"
	HeaderSetCookie      = "Set-Cookie"
	HeaderSetCookieLower = "set-cookie"

	// Connection management
	HeaderConnection      = "Connection"
	HeaderKeepAlive       = "Keep-Alive"
	HeaderProxyConnection = "Proxy-Connection"

	// Authentication
	HeaderAuthorization      = "Authorization"
	HeaderProxyAuthenticate  = "Proxy-Authenticate"
	HeaderProxyAuthorization = "Proxy-Authorization"
	HeaderWWWAuthenticate    = "WWW-Authenticate"

	// Range requests
	HeaderAcceptRanges = "Accept-Ranges"
	HeaderContentRange = "Content-Range"
	HeaderIfRange      = "If-Range"
	HeaderRange        = "Range"

	// Response context
	HeaderAllow       = "Allow"
	HeaderServer      = "Server"
	HeaderServerLower = "server"

	// Request context
	HeaderFrom           = "From"
	HeaderHost           = "Host"
	HeaderReferer        = "Referer"
	HeaderReferrerPolicy = "Referrer-Policy"
	HeaderUserAgent      = "User-Agent"

	// Message body information
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLanguage = "Content-Language"
	HeaderContentLength   = "Content-Length"
	HeaderContentLocation = "Content-Location"
	HeaderContentType     = "Content-Type"

	// Content negotiation
	HeaderAccept         = "Accept"
	HeaderAcceptCharset  = "Accept-Charset"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAcceptLanguage = "Accept-Language"
	HeaderAltSvc         = "Alt-Svc"

	// Protocol
	HTTP11 = "HTTP/1.1"
	HTTP10 = "HTTP/1.0"
	HTTP20 = "HTTP/2.0"

	// MIME text
	MIMETextPlain             = "text/plain"
	MIMETextPlainUTF8         = "text/plain; charset=utf-8"
	MIMETextPlainISO88591     = "text/plain; charset=iso-8859-1"
	MIMETextPlainFormatFlowed = "text/plain; format=flowed"
	MIMETextPlainDelSpaceYes  = "text/plain; delsp=yes"
	MiMETextPlainDelSpaceNo   = "text/plain; delsp=no"
	MIMETextHtml              = "text/html"
	MIMETextCss               = "text/css"
	MIMETextJavascript        = "text/javascript"
	MIMEMultipartPOSTForm     = "multipart/form-data"

	// MIME application
	MIMEApplicationOctetStream  = "application/octet-stream"
	MIMEApplicationFlash        = "application/x-shockwave-flash"
	MIMEApplicationHTMLForm     = "application/x-www-form-urlencoded"
	MIMEApplicationHTMLFormUTF8 = "application/x-www-form-urlencoded; charset=UTF-8"
	MIMEApplicationTar          = "application/x-tar"
	MIMEApplicationGZip         = "application/gzip"
	MIMEApplicationXGZip        = "application/x-gzip"
	MIMEApplicationBZip2        = "application/bzip2"
	MIMEApplicationXBZip2       = "application/x-bzip2"
	MIMEApplicationShell        = "application/x-sh"
	MIMEApplicationDownload     = "application/x-msdownload"
	MIMEApplicationJSON         = "application/json"
	MIMEApplicationJSONUTF8     = "application/json; charset=utf-8"
	MIMEApplicationXML          = "application/xml"
	MIMEApplicationXMLUTF8      = "application/xml; charset=utf-8"
	MIMEApplicationZip          = "application/zip"
	MIMEApplicationPdf          = "application/pdf"
	MIMEApplicationWord         = "application/msword"
	MIMEApplicationExcel        = "application/vnd.ms-excel"
	MIMEApplicationPPT          = "application/vnd.ms-powerpoint"
	MIMEApplicationOpenXMLWord  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEApplicationOpenXMLExcel = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MIMEApplicationOpenXMLPPT   = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	MIMEPROTOBUF                = "application/x-protobuf"

	// MIME image
	MIMEImageJPEG         = "image/jpeg"
	MIMEImagePNG          = "image/png"
	MIMEImageGIF          = "image/gif"
	MIMEImageBitmap       = "image/bmp"
	MIMEImageWebP         = "image/webp"
	MIMEImageIco          = "image/x-icon"
	MIMEImageMicrosoftICO = "image/vnd.microsoft.icon"
	MIMEImageTIFF         = "image/tiff"
	MIMEImageSVG          = "image/svg+xml"
	MIMEImagePhotoshop    = "image/vnd.adobe.photoshop"

	// MIME audio
	MIMEAudioBasic     = "audio/basic"
	MIMEAudioL24       = "audio/L24"
	MIMEAudioMP3       = "audio/mp3"
	MIMEAudioMP4       = "audio/mp4"
	MIMEAudioMPEG      = "audio/mpeg"
	MIMEAudioOggVorbis = "audio/ogg"
	MIMEAudioWAVE      = "audio/vnd.wave"
	MIMEAudioWebM      = "audio/webm"
	MIMEAudioAAC       = "audio/x-aac"
	MIMEAudioAIFF      = "audio/x-aiff"
	MIMEAudioMIDI      = "audio/x-midi"
	MIMEAudioM3U       = "audio/x-mpegurl"
	MIMEAudioRealAudio = "audio/x-pn-realaudio"

	// MIME video
	MIMEVideoMPEG          = "video/mpeg"
	MIMEVideoOgg           = "video/ogg"
	MIMEVideoMP4           = "video/mp4"
	MIMEVideoQuickTime     = "video/quicktime"
	MIMEVideoWinMediaVideo = "video/x-ms-wmv"
	MIMEVideWebM           = "video/webm"
	MIMEVideoFlashVideo    = "video/x-flv"
	MIMEVideo3GPP          = "video/3gpp"
	MIMEVideoAVI           = "video/x-msvideo"
	MIMEVideoMatroska      = "video/x-matroska"
)
