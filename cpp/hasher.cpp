#include<iostream>
#include<fstream>
#include <sstream>
#include <iomanip>
#include <map>
#include<boost/filesystem.hpp>
#include<boost/uuid/sha1.hpp>

// Value-Defintions of the different String values

std::string digest_to_str(unsigned int digest[5])
{
  std::stringstream ss;
  ss << std::hex << std::setfill('0') << std::setw(8) << digest[0];
  ss << std::hex << std::setfill('0') << std::setw(8) << digest[1];
  ss << std::hex << std::setfill('0') << std::setw(8) << digest[2];
  ss << std::hex << std::setfill('0') << std::setw(8) << digest[3];
  ss << std::hex << std::setfill('0') << std::setw(8) << digest[4];
  return ss.str();
}

std::string sha1_file(std::ifstream& file, std::streampos start)
{
  std::streampos bs;
  boost::uuids::detail::sha1 s;
  bs = 16384;
  char * memblock;
  memblock = new char [bs];
  file.seekg (start, std::ios::beg);
  while (file) {
    file.read (memblock, bs);
    if (file)
      s.process_bytes(memblock, bs); 
    else if (file.eof())
      s.process_bytes(memblock, file.gcount());
    else
      s.reset();
  }
  unsigned int digest[5];
  s.get_digest(digest);
  delete[] memblock;
  return digest_to_str(digest);
}

std::string nes_sha1(std::ifstream& file)
{
  boost::uuids::detail::sha1 s;
  char * b = new char [16];
  file.seekg (0, std::ios::beg);
  file.read(b, 16);
  unsigned char* header = reinterpret_cast<unsigned char*>(b);
  unsigned int prg_size = (unsigned int)header[4];
  unsigned int chr_size = (unsigned int)header[5];
  if ((header[7] & 12) == 8) {
    unsigned char rom_size = (unsigned int)header[9];
    chr_size = ((rom_size & 0x0F) << 8) + chr_size;
    prg_size = ((rom_size & 0xF0) << 4) + prg_size;
  }
  unsigned int prg, chr, start;
  prg = 16 * 1024 * prg_size;
  chr = 8 * 1024 * chr_size;
  start = 16;
  if ((header[6] & 4) == 4) {
    start += 512;
  }
  
  char * rom_data = new char [chr + prg];
  file.seekg (start, std::ios::beg);
  file.read (rom_data, chr + prg);
  if (file)
    s.process_bytes(rom_data, chr + prg); 
 //   else if (file.eof())
 //     s.process_bytes(memblock, file.gcount());
    else
      s.reset();
  //}
  unsigned int digest[5];
  s.get_digest(digest);
  delete[] rom_data;
  return digest_to_str(digest);
}

void deinterleave(char* p, unsigned int n) {
	unsigned int m, i;
        m = n / 2;
	char * b = new char [n];
        for (i=0 ; i<n ; i++) {
		if (i < m) {
			b[i*2+1] = p[i];
		} else {
			b[i*2-n] = p[i];
		}
	}
        for (i=0 ; i<n; i++) {
          p[i] = b[i];
        }
        delete[] b;
}

std::string block_sha1(std::ifstream& file, unsigned int bs, void (*mod)(char*, unsigned int))
{
  boost::uuids::detail::sha1 s;
  char * memblock;
  memblock = new char [bs];
  file.seekg (0, std::ios::beg);
  while (file) {
    file.read (memblock, bs);
    if (file) {
      mod(memblock, bs);
      s.process_bytes(memblock, bs);
    }
    else if (file.eof())
      break;
    else s.reset();
  }
  unsigned int digest[5];
  s.get_digest(digest);
  delete[] memblock;
  return digest_to_str(digest);
}

void noop(char* b, unsigned int n) {
  (void)b;
  (void)n;
}

void n_swap(char* b, unsigned int n) {
  char tmp;
  unsigned int i;
  if (n % 4 != 0) {
    return;
  }
  for (i = 0; i < n; i += 4) {
    tmp = b[i+2];
    b[i+2] = b[i];
    b[i] = tmp;
    tmp = b[i+3];
    b[i+3] = b[i+1];
    b[i+1] = tmp;
  }
}

void z_swap(char* b, unsigned int n) {
  char tmp;
  unsigned int i;
  if (n % 4 != 0) {
    return;
  }
  for (i = 0; i < n; i += 4) {
    tmp = b[i+1];
    b[i+1] = b[i];
    b[i] = tmp;
    tmp = b[i+3];
    b[i+3] = b[i+2];
    b[i+2] = tmp;
  }
}

std::string n64_sha1(std::ifstream& file)
{
  char * b = new char [4];
  void (*swap)(char*, unsigned int);
  file.seekg (0, std::ios::beg);
  file.read (b, 4);
  unsigned char* header = reinterpret_cast<unsigned char*>(b);
  if (header[0] == 0x80) {
    swap = &z_swap;
  } else if (header[3] == 0x80) {
    swap = &n_swap;
  } else {
    swap = &noop;
  }
  return block_sha1(file, 16384, swap);
} 

class Hasher {
  enum Decoder { NotDefined,
                 Binary,
                 SNES,
                 SegaMGD,
                 SegaSMD,
                 LNX,
                 N64,
                 NES };
  std::map<boost::filesystem::path,Decoder> decoders;
  public:
    Hasher ();
    std::string SHA1 (std::string);
};

Hasher::Hasher() {
  decoders[".bin"] = Binary;
  decoders[".32x"] = Binary;
  decoders[".a26"] = Binary;
  decoders[".bin"] = Binary;
  decoders[".gb"] = Binary;
  decoders[".gbc"] = Binary;
  decoders[".gba"] = Binary;
  decoders[".gen"] = Binary;
  decoders[".gg"] = Binary;
  decoders[".md"] = Binary;
  decoders[".pce"] = Binary;
  decoders[".rom"] = Binary;
  decoders[".sms"] = Binary;
  decoders[".fig"] = SNES;
  decoders[".sfc"] = SNES;
  decoders[".smc"] = SNES;
  decoders[".swc"] = SNES;
  // TODO: Switch to parsing ROM header to detect interleaving instead of extension.
  decoders[".mgd"] = SegaMGD;
  decoders[".smd"] = SegaSMD;
  decoders[".lnx"] = LNX;
  decoders[".lyx"] = LNX;
  decoders[".n64"] = N64;
  decoders[".v64"] = N64;
  decoders[".z64"] = N64;
  decoders[".nes"] = NES;
}

std::string Hasher::SHA1(std::string fp) {
  std::streampos size;
  boost::filesystem::path ext;
  boost::filesystem::path p = fp;
  std::ifstream file (p.c_str(), std::ios::binary | std::ios::ate);
  char * b;
  std::string c;
  if (file.is_open()) {
    size = file.tellg();
    ext = p.extension(); // outputs ".txt"
    switch (decoders[ext])
    {
      case Binary:
        return sha1_file(file, 0);
      case SNES:
        if (size % 1024 == 512)
          return sha1_file(file, 512);
        else
          return sha1_file(file, 0);
        break;
      case SegaMGD:
        return block_sha1(file, size, deinterleave);
      case SegaSMD:
        return block_sha1(file, 16384, deinterleave);
      case LNX:
	b = new char [4];
        file.read(b, 4);
	c = std::string(b);
        if (c == "LYNX")
          return sha1_file(file, 64);
        else
          return sha1_file(file, 0);
      case N64:
        return n64_sha1(file); 
      case NES:
        return nes_sha1(file); 
      case NotDefined:
        return "";
    }
  }
  file.close();
  return "";
}


int main(int argc, char *argv[])
{
Hasher h;
for (int i = 1; i < argc; i++) {
  std::cout << h.SHA1(argv[i]);
  std::cout << " ";
  std::cout << argv[i];
  std::cout << "\n";
}
return(0);
}
