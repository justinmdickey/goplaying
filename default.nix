{ buildGoModule, lib, makeWrapper, playerctl, }: buildGoModule {
  pname = "goplaying";
  version = "0.1.0+nightly";
  
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./main.go
    ];
  };

  vendorHash = "sha256-0SK2yDnLt1fEp6nKjQYDs/pEWwCV96pWMkKZXvax2Ds=";
  nativeBuildInputs = [makeWrapper];

  postInstall = ''
    wrapProgram $out/bin/goplaying \
      --prefix PATH : "${lib.makeBinPath [playerctl]}"
  '';

  meta = {
    description = "Basic now-playing TUI written in Go";
    homepage = "https://github.com/justinmdickey/goplaying";
    license = lib.licenses.mit;
  };
}
