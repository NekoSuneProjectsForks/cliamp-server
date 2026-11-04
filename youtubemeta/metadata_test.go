package youtubemeta

import "testing"

func TestCleanYouTubeTitles(t *testing.T) {
 cases:=[]struct{
  input,artist,title string
 }{
  {"THIRST - MAYHEM [Drift Phonk | Monstercat]","THIRST","MAYHEM"},
  {"THIRST – MAYHEM [Drift Phonk] [Monstercat]","THIRST","MAYHEM"},
  {"Artist — Track Name (Official Music Video)","Artist","Track Name"},
  {"Artist - Track [Live] (Official Audio)","Artist","Track"},
  {"Artist - My [Favourite] Track","Artist","My [Favourite] Track"},
  {"Artist - B-Side","Artist","B-Side"},
  {"No separator [HD]","","No separator"},
 }
 for _,tc:=range cases {
  got:=Clean(tc.input,"","")
  if got.Artist!=tc.artist || got.Title!=tc.title||got.RawTitle!=tc.input {
   t.Errorf("Clean(%q) = %+v, expected %q - %q",tc.input,got,tc.artist,tc.title)
  }
 }
}
func TestCleanFallbackMetadata(t *testing.T){
 got:=Clean("The Song [Official Audio]","Performer","")
 if got.Artist!="Performer"||got.Title!="The Song"{t.Fatalf("unexpected fallback %+v",got)}
 got=Clean("Artist - Title [Official Video]","ChannelUploader","")
 if got.Artist!="Artist"||got.Title!="Title"{t.Fatalf("video title should win over channel: %+v",got)}
}
func TestLookupRejectsUntrustedURLs(t *testing.T){
 _,err:=Lookup(t.Context(),"https://example.com/watch?v=AMmBY2D9IJU")
 if err==nil{t.Fatal("untrusted URL accepted")}
}

func TestPrintedMetadataFields(t *testing.T){
 input:="\"THIRST - MAYHEM [Drift Phonk | Monstercat]\"\n\"NA\"\n\"NA\"\n\"NA\"\n\"NA\"\n\"unlisted\"\n"
 fields,err:=parsePrintedFields([]byte(input));if err!=nil{t.Fatal(err)}
 if fields[0]!="THIRST - MAYHEM [Drift Phonk | Monstercat]"||fields[1]!=""{t.Fatalf("bad field parsing: %#v",fields)}
 got:=Clean(fields[0],fields[1],fields[2])
 if got.Artist!="THIRST"||got.Title!="MAYHEM"{t.Fatalf("bad title parsing: %+v",got)}
}

func TestVideoFailureClassificationNeverMistakesNetworkForDeleted(t *testing.T){
 cases:=[]struct{message,want string}{
  {"ERROR: [youtube] abc: This video has been removed by the uploader","deleted"},
  {"ERROR: [youtube] abc: Private video. Sign in to view","private"},
  {"ERROR: HTTP Error 429: Too Many Requests","unknown"},
  {"ERROR: Video unavailable","unknown"},
  {"ERROR: Sign in to confirm you're not a bot","unknown"},
  {"ERROR: This video is unlisted","unknown"},
  {"ERROR: This video is unavailable in your country","unknown"},
  {"ERROR: Connection timed out","unknown"},
 }
 for _,tc:=range cases{if got:=classifyFailure(tc.message);got!=tc.want{t.Errorf("classifyFailure(%q)=%q, want %q",tc.message,got,tc.want)}}
}

func TestParseBareYTDLPMissingFields(t *testing.T) {
 output := []byte("\"THIRST - MAYHEM [Drift Phonk | Monstercat]\"\nNA\nNA\nNA\nNA\n\"public\"\n")
 fields,err:=parsePrintedFields(output)
 if err!=nil{t.Fatal(err)}
 if fields[0]!="THIRST - MAYHEM [Drift Phonk | Monstercat]"||fields[1]!=""||fields[4]!=""||fields[5]!="public"{t.Fatalf("unexpected values: %#v",fields)}
 cleaned:=Clean(fields[0],fields[1],fields[2])
 if cleaned.Artist!="THIRST"||cleaned.Title!="MAYHEM"{t.Fatalf("bad metadata: %+v",cleaned)}
}
func TestParseYTDLPMissingNullAndInvalidValues(t *testing.T) {
 if _,err:=parsePrintedFields([]byte("\"Track\"\nnull\nNA\n\"\"\nNA\n\"unlisted\"\n"));err!=nil{t.Fatal(err)}
 if _,err:=parsePrintedFields([]byte("\"Track\"\nNOT_JSON\nNA\nNA\nNA\nNA\n"));err==nil{t.Fatal("invalid non-NA value accepted")}
}
