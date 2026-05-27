cask "lumen-gateway" do
  version "1.2.2"
  sha256 :no_check

  url "https://github.com/EdwinZhanCN/Lumen-SDK/releases/download/v#{version}/lumen-gateway-#{version}-darwin-universal.zip"
  name "Lumen Gateway"
  desc "Lumen Distributed AI Inference Gateway"
  homepage "https://github.com/EdwinZhanCN/Lumen-SDK"

  app "Lumen Gateway.app"

  zap trash: "~/Library/Application Support/Lumen Gateway"
end
