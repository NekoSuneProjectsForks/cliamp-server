package playlist

import (
 "testing"
 "cliamp-server/library"
)

func TestReplaceRotation(t *testing.T) {
 p:=New([]library.Track{{Path:"old.mp3",Title:"Old"}},false)
 if got:=p.Next().Path;got!="old.mp3"{t.Fatalf("got %q",got)}
 if err:=p.Replace([]library.Track{{Path:"new.mp3",Title:"New"},{Path:"new2.mp3",Title:"New2"}});err!=nil{t.Fatal(err)}
 if p.Len()!=2 {t.Fatalf("unexpected track count: %d",p.Len())}
 if got:=p.Next().Path;got!="new.mp3"{t.Fatalf("updated rotation got %q",got)}
 if err:=p.Replace(nil);err==nil {t.Fatal("empty library should be rejected")}
 if p.Len()!=2 {t.Fatal("empty replacement changed active rotation")}
}
