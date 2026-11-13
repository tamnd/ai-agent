FROM eclipse-temurin:25-jdk-noble@sha256:95e350ad1eb738f943e141640f7e3d797e483359a3b33c3864255bfd1ad71f8a
ADD --checksum=sha256:3fdfc37e29744a9a67dd5365e81467e26fbde0b7aa204e6f8bbe79eeaa7ae892 https://repo1.maven.org/maven2/org/junit/platform/junit-platform-console-standalone/1.13.4/junit-platform-console-standalone-1.13.4.jar /opt/algo/junit.jar
ADD --checksum=sha256:7cc140e76daf416a0424a5557f99afd7ca89ebd463ea1101261e3e01c33e75cd https://github.com/JetBrains/kotlin/releases/download/v2.4.21/kotlin-compiler-2.4.21.zip /tmp/kotlin.zip
# The JDK reads a zip file with jar, so the image needs no unzip.
RUN cd /opt && jar xf /tmp/kotlin.zip && rm /tmp/kotlin.zip && chmod +x /opt/kotlinc/bin/* && chmod a+r /opt/algo/junit.jar
ENV PATH=/opt/kotlinc/bin:$PATH
USER 1000:1000
