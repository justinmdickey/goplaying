{ buildGoModule, lib, makeWrapper, playerctl, }:
buildGoModule {
  pname = "goplaying";
  version = "0.1.0+nightly.20250205";

  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./main.go
    ];
  };

  vendorHash = "sha256-TYd6Eo2bS4AnKqDAquCcMBm6ihOJEK2ak+RWKfIDspY=";
  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/goplaying \
      --prefix PATH : "${lib.makeBinPath [ playerctl ]}"
  '';

  meta = {
    description = "Basic now-playing TUI written in Go";
    homepage = "https://github.com/justinmdickey/goplaying";
    license = lib.licenses.mit;
  };
}
