use strict;
use warnings;

my $method = $ENV{'REQUEST_METHOD'} // '';
my $query = $ENV{'QUERY_STRING'} // '';

print "Content-Type: text/html\n";
print "\n";

print "<h1>Hello from Perl CGI!</h1>\n";
print "<p>Method: $method</p>\n";
print "<p>Query: $query</p>\n";
