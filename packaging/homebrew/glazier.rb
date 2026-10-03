# The release workflow fills the @...@ placeholders with packaging/render.sh and pushes the result to the tap.
class Glazier < Formula
  desc "Declarative tmux workspaces from HCL profiles"
  homepage "https://github.com/wilhelm-murdoch/glazier"
  version "@VERSION@"
  license "MIT"

  depends_on "tmux"

  on_macos do
    on_arm do
      url "https://github.com/wilhelm-murdoch/glazier/releases/download/v#{version}/glaze-darwin-arm64.zip"
      sha256 "@SHA256_glaze-darwin-arm64.zip@"
    end

    on_intel do
      url "https://github.com/wilhelm-murdoch/glazier/releases/download/v#{version}/glaze-darwin-amd64.zip"
      sha256 "@SHA256_glaze-darwin-amd64.zip@"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/wilhelm-murdoch/glazier/releases/download/v#{version}/glaze-linux-arm64.zip"
      sha256 "@SHA256_glaze-linux-arm64.zip@"
    end

    on_intel do
      url "https://github.com/wilhelm-murdoch/glazier/releases/download/v#{version}/glaze-linux-amd64.zip"
      sha256 "@SHA256_glaze-linux-amd64.zip@"
    end
  end

  def install
    bin.install "glaze"
  end

  test do
    assert_match "Version: v#{version},", shell_output("#{bin}/glaze --version")
  end
end
