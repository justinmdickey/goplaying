{
  description = "Now Playing TUI written in Go";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    systems.url = "github:nix-systems/default-linux";
  };

  outputs = {nixpkgs, systems,...}:
  let
    forEachSys = f: nixpkgs.lib.genAttrs
      (import systems)
      (sys: f nixpkgs.legacyPackages.${sys});
  in {
    packages = forEachSys (pkgs: rec {
      goplaying = pkgs.callPackage ./default.nix {};
      default = goplaying;
    });
  };
}
